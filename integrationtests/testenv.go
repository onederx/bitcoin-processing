package integrationtests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const networkName = "bitcoin-processing-integration-test-network"

type containerInfo struct {
	name string
	id   string
	ip   string
}

type testEnvironment struct {
	cli            *client.Client
	network        string
	networkGateway string

	db *containerInfo

	regtest         map[string]*containerInfo
	regtestIsLoaded chan error

	processing           *containerInfo
	processingConfigPath string
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
	cli, err := client.NewEnvClient()
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

	if len(foundErrors) > 0 {
		return errors.New(strings.Join(foundErrors, " "))
	}
	return nil
}

func (e *testEnvironment) waitForLoad() {
	e.waitForDatabase()
	e.waitForRegtest()
}

func (e *testEnvironment) processingUrl(relative string) string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:8000", e.processing.ip),
		Path:   relative,
	}).String()
}
