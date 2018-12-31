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
	cli     *client.Client
	network string

	db *containerInfo

	regtest         map[string]*containerInfo
	regtestIsLoaded chan error

	processing           *containerInfo
	processingConfigPath string
}

func setupNetwork(ctx context.Context, cli *client.Client, network string) error {
	resp, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for i := range resp {
		if resp[i].Name == network {
			return nil
		}
	}

	_, err = cli.NetworkCreate(ctx, network, types.NetworkCreate{})
	return err
}

func newDockerClient(ctx context.Context) (*client.Client, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(ctx)
	if err := setupNetwork(ctx, cli, networkName); err != nil {
		return nil, err
	}
	return cli, nil
}

func newTestEnvironment(ctx context.Context) (*testEnvironment, error) {
	cli, err := newDockerClient(ctx)
	if err != nil {
		return nil, err
	}
	env := &testEnvironment{
		cli:     cli,
		network: networkName,
	}
	return env, nil
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
