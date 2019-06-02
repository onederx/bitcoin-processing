package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/onederx/bitcoin-processing/storage"
)

func main() {
	settings.ReadSettingsAndRun(func(loadedSettings settings.Settings) {
		nodeAPI := nodeapi.NewNodeAPI(loadedSettings)
		db := storage.Open(loadedSettings)

		eventBroker := events.NewEventBroker(
			loadedSettings,
			events.NewEventStorage(db),
		)
		bitcoinWallet := wallet.NewWallet(
			loadedSettings,
			nodeAPI,
			eventBroker,
			wallet.NewStorage(db),
		)
		apiServer := api.NewServer(
			loadedSettings.GetString("api.http.address"),
			bitcoinWallet,
			eventBroker,
			loadedSettings.GetBool("wallet.allow_withdrawal_without_id"),
		)

		bitcoinWallet.Check()
		eventBroker.Check()

		eventBrokerStopped := make(chan struct{})
		walletStopped := make(chan struct{})
		apiServerStopped := make(chan struct{})

		runner := newProcessingComponentRunner()

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			select {
			case <-eventBrokerStopped:
			case <-walletStopped:
			case <-apiServerStopped:
			case signal := <-signals:
				log.Printf("Received signal %v", signal)
			}

			// shut down everything

			log.Printf("Bitcoin processing is stopping")

			bitcoinWallet.Stop()
			eventBroker.Stop()
			apiServer.Stop()
		}()

		runner.run(apiServer, "API server", apiServerStopped)
		runner.run(bitcoinWallet, "Wallet", walletStopped)
		runner.run(eventBroker, "Event broker", eventBrokerStopped)

		runner.wait()

		if runner.failed {
			log.Fatal("Bitcoin processing has stopped abnormally")
		}

		log.Printf("Bitcoin processing has normally stopped")
	})
}
