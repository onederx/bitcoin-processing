package testenv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	processingapiclient "github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

const (
	networkName = "bitcoin-processing-integration-test-network"

	listenersMessageQueueSize   = 20000
	listenersMessageWaitTimeout = time.Minute
)

type containerInfo struct {
	ID   string
	name string
	IP   string
}

type bitcoinNodeContainerInfo struct {
	containerInfo
	NodeAPI nodeapi.NodeAPI
}

type TestEnvironment struct {
	cli            *dockerclient.Client
	network        string
	NetworkGateway string

	DB *containerInfo

	Regtest          map[string]*bitcoinNodeContainerInfo
	regtestIsLoaded  chan error
	notifyScriptFile *os.File

	Processing           *containerInfo
	ProcessingSettings   *ProcessingSettings
	processingConfigPath string
	ProcessingClient     *processingapiclient.Client

	callbackListener     *httptest.Server
	CallbackURL          string
	callbackMessageQueue chan *callbackRequest
	CallbackHandler      func(http.ResponseWriter, *http.Request)

	WebsocketListeners []*WebsocketListener

	depositFee bitcoin.BTCAmount
}

func NewTestEnvironment(ctx context.Context, depositFee bitcoin.BTCAmount) (*TestEnvironment, error) {
	env := &TestEnvironment{depositFee: depositFee}

	err := env.setupDockerClient(ctx)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (e *TestEnvironment) setupDockerClient(ctx context.Context) error {
	cli, err := dockerclient.NewEnvClient()
	if err != nil {
		return err
	}
	e.cli = cli
	cli.NegotiateAPIVersion(ctx)
	if err := e.setupNetwork(ctx); err != nil {
		return err
	}
	return nil
}

func (e *TestEnvironment) setupNetwork(ctx context.Context) error {
	e.network = networkName

	resp, err := e.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for i := range resp {
		if resp[i].Name == networkName {
			e.NetworkGateway = resp[i].IPAM.Config[0].Gateway
			return nil
		}
	}

	netInfo, err := e.cli.NetworkCreate(ctx, networkName, types.NetworkCreate{})

	if err != nil {
		return err
	}

	netResource, err := e.cli.NetworkInspect(ctx, netInfo.ID, types.NetworkInspectOptions{})

	if err != nil {
		return err
	}

	e.NetworkGateway = netResource.IPAM.Config[0].Gateway

	return nil
}

func (e *TestEnvironment) getContainerIP(ctx context.Context, id string) string {
	resp, err := e.cli.ContainerInspect(ctx, id)
	if err != nil {
		panic(err)
	}

	for name, settings := range resp.NetworkSettings.Networks {
		if name == e.network {
			return settings.IPAddress
		}
	}

	panic("Failed to find container ip in network " + e.network)
}

func (e *TestEnvironment) Start(ctx context.Context) error {
	err := e.StartDatabase(ctx)
	if err != nil {
		return err
	}
	err = e.startRegtest(ctx, e.depositFee)
	if err != nil {
		return err
	}
	e.startCallbackListener()
	return nil
}

func (e *TestEnvironment) Stop(ctx context.Context) error {
	foundErrors := make([]string, 0)

	dbStopErr := e.stopDatabase(ctx)

	if dbStopErr != nil {
		errorMsg := fmt.Sprintf("Error stopping database container: %s.", dbStopErr)
		log.Printf(errorMsg)
		foundErrors = append(foundErrors, errorMsg)
	}

	regtestStopErr := e.stopRegtest(ctx)

	if regtestStopErr != nil {
		errorMsg := fmt.Sprintf("Error stopping regtest containers: %s.", regtestStopErr)
		log.Printf(errorMsg)
		foundErrors = append(foundErrors, errorMsg)
	}

	e.stopCallbackListener()

	for _, wsListener := range e.WebsocketListeners {
		wsListener.Stop()
	}
	e.WebsocketListeners = nil

	if len(foundErrors) > 0 {
		return errors.New(strings.Join(foundErrors, " "))
	}
	return nil
}

func (e *TestEnvironment) WaitForLoad() {
	e.WaitForDatabase()
	e.waitForRegtest()
}

func (e *TestEnvironment) processingURL(relative string) string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:8000", e.Processing.IP),
		Path:   relative,
	}).String()
}
