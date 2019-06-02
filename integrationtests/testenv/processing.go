package testenv

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"

	"github.com/onederx/bitcoin-processing/api/client"
)

const (
	baseImageName           = "debian:stretch"
	processingContainerName = "bitcoin-processing-integration-test-main"

	DefaultCallbackURLPath = "/wallets/cb"

	configTemplate = `transaction:
  callback:
    url: {{.CallbackURL}}
  max_confirmations: {{.MaxConfirmations}}
api:
  http:
    address: 0.0.0.0:8000
storage:
  type: postgres
  dsn: >
    host={{.PostgresAddress}} dbname=bitcoin_processing
    user=bitcoin_processing sslmode=disable
bitcoin:
  node:
    address: bitcoin-processing-integration-test-node-our:18443
    user: bitcoinrpc
    password: TEST_BITCOIN_NODE_PASSWORD
wallet:
   min_withdraw_without_manual_confirmation: {{.MinWithdrawWithoutManualConfirmation}}
   {{.AdditionalWalletSettings}}`
)

type processingSettings struct {
	MaxConfirmations                     int
	CallbackURL                          string
	MinWithdrawWithoutManualConfirmation string
	AdditionalWalletSettings             string
	PostgresAddress                      string
}

var DefaultSettings = processingSettings{
	MaxConfirmations:                     1,
	CallbackURL:                          "http://127.0.0.1:9000" + DefaultCallbackURLPath,
	MinWithdrawWithoutManualConfirmation: "0.1",
	PostgresAddress:                      "bitcoin-processing-integration-test-db",
}

func (e *TestEnvironment) StartProcessingWithDefaultSettings(ctx context.Context) error {
	settings := DefaultSettings
	if e.CallbackURL != "" {
		settings.CallbackURL = e.CallbackURL
	}
	return e.StartProcessing(ctx, &settings)
}

func (e *TestEnvironment) StartProcessing(ctx context.Context, s *processingSettings) error {
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
	e.Processing = &containerInfo{name: "main", ID: resp.ID}

	err = e.cli.ContainerStart(ctx, e.Processing.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	e.Processing.ip = e.getContainerIP(ctx, resp.ID)

	e.setProcessingAddressForNotifications(e.Processing.ip)

	e.ProcessingClient = client.NewClient(e.processingURL("/"))

	e.ProcessingSettings = s

	log.Printf("processing container started: id=%v", e.Processing.ID)

	err = e.writeContainerLogs(ctx, e.Processing, "processing.log")

	if err != nil {
		return err
	}

	return nil
}

func (e *TestEnvironment) StopProcessing(ctx context.Context) error {
	log.Printf("stopping bitcoin processing container")

	if e.Processing == nil {
		log.Printf("seems that processing is not running")
		return nil
	}

	if err := e.cli.ContainerStop(ctx, e.Processing.ID, nil); err != nil {
		return err
	}

	log.Printf("bitcoin processing container stopped: id=%v", e.Processing.ID)
	e.Processing = nil
	return nil
}

func (e *TestEnvironment) WaitForProcessing() {
	log.Printf("waiting for processing to start")
	waitForPort(e.Processing.ip, 8000)
	log.Printf("processing started")
}
