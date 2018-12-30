package integrationtests

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
)

const (
	baseImageName           = "debian:stretch"
	processingContainerName = "bitcoin-processing-integration-test-main"

	configTemplate = `transaction:
  callback:
    url: http://127.0.0.1:9000/
  max_confirmations: {{.MaxConfirmations}}
api:
  http:
    address: 0.0.0.0:8000
storage:
  type: postgres
  dsn: >
    host=bitcoin-processing-integration-test-db dbname=bitcoin_processing
    user=bitcoin_processing sslmode=disable
bitcoin:
  node:
    address: bitcoin-processing-integration-test-node-our:18443
    user: bitcoinrpc
    password: TEST_BITCOIN_NODE_PASSWORD`
)

type processingSettings struct {
	MaxConfirmations int
}

var defaultSettings = processingSettings{
	MaxConfirmations: 1,
}

func (e *testEnvironment) startProcessingWithDefaultSettings(ctx context.Context) error {
	return e.startProcessing(ctx, &defaultSettings)
}

func (e *testEnvironment) startProcessing(ctx context.Context, s *processingSettings) error {
	log.Printf("Starting bitcoin processing container")

	configTempFile, err := ioutil.TempFile("", "")

	if err != nil {
		return err
	}

	configTempFilePath := configTempFile.Name()

	defer os.Remove(configTempFilePath)
	defer configTempFile.Close()

	tmpl := template.Must(template.New("config").Parse(configTemplate))
	tmpl.Execute(configTempFile, s)

	containerConfig := &container.Config{
		Image:      baseImageName,
		Entrypoint: strslice.StrSlice{"/bitcoin-processing", "-c", "/config.yml"},
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(e.network),
		AutoRemove:  true,
		Binds: []string{
			getFullSourcePath("cmd/bitcoin-processing/bitcoin-processing") + ":/bitcoin-processing",
			configTempFilePath + ":/config.yml",
		},
	}

	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, processingContainerName)
	if err != nil {
		return err
	}
	e.processing = &containerInfo{
		name: "main",
		id:   resp.ID,
	}

	err = e.cli.ContainerStart(ctx, e.processing.id, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	e.processing.ip = e.getContainerIP(ctx, resp.ID)

	log.Printf("processing container started: id=%v", e.processing.id)
	return nil
}

func (e *testEnvironment) stopProcessing(ctx context.Context) error {
	log.Printf("stopping bitcoin processing container")

	if err := e.cli.ContainerStop(ctx, e.processing.id, nil); err != nil {
		return err
	}

	log.Printf("bitcoin processing container stopped: id=%v", e.processing.id)
	return nil
}

func (e *testEnvironment) waitForProcessing() {
	log.Printf("waiting for processing to start")
	waitForPort(e.processing.ip, 8000)
	log.Printf("processing started")
}
