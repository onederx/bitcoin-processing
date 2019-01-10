package integrationtests

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
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

const (
	networkName = "bitcoin-processing-integration-test-network"

	listenersMessageQueueSize   = 20000
	listenersMessageWaitTimeout = time.Minute
)

type containerInfo struct {
	name string
	id   string
	ip   string
}

type bitcoinNodeContainerInfo struct {
	containerInfo
	nodeAPI nodeapi.NodeAPI
}

type testEnvironment struct {
	cli            *dockerclient.Client
	network        string
	networkGateway string

	db *containerInfo

	regtest          map[string]*bitcoinNodeContainerInfo
	regtestIsLoaded  chan error
	notifyScriptFile *os.File

	processing           *containerInfo
	processingSettings   *processingSettings
	processingConfigPath string
	processingClient     *processingapiclient.Client

	callbackListener     *httptest.Server
	callbackURL          string
	callbackMessageQueue chan *callbackRequest
	callbackHandler      func(http.ResponseWriter, *http.Request)

	websocketListeners []*websocketListener
}

func newTestEnvironment(ctx context.Context) (*testEnvironment, error) {
	env := &testEnvironment{}

	err := env.setupDockerClient(ctx)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (e *testEnvironment) setupDockerClient(ctx context.Context) error {
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

func (e *testEnvironment) setupNetwork(ctx context.Context) error {
	e.network = networkName

	resp, err := e.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for i := range resp {
		if resp[i].Name == networkName {
			e.networkGateway = resp[i].IPAM.Config[0].Gateway
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

	e.networkGateway = netResource.IPAM.Config[0].Gateway

	return nil
}

func (e *testEnvironment) getContainerIP(ctx context.Context, id string) string {
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

func (e *testEnvironment) start(ctx context.Context) error {
	err := e.startDatabase(ctx)
	if err != nil {
		return err
	}
	err = e.startRegtest(ctx)
	if err != nil {
		return err
	}
	e.startCallbackListener()
	return nil
}

func (e *testEnvironment) stop(ctx context.Context) error {
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

	for _, wsListener := range e.websocketListeners {
		wsListener.stop()
	}
	e.websocketListeners = nil

	if len(foundErrors) > 0 {
		return errors.New(strings.Join(foundErrors, " "))
	}
	return nil
}

func (e *testEnvironment) waitForLoad() {
	e.waitForDatabase()
	e.waitForRegtest()
}

func (e *testEnvironment) processingURL(relative string) string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:8000", e.processing.ip),
		Path:   relative,
	}).String()
}
