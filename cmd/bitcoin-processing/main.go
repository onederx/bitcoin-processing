package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
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

		wg := sync.WaitGroup{}
		wg.Add(3)

		go func() {
			apiServer.Run()
			log.Print("API server has stopped")
			close(eventBrokerStopped)
			wg.Done()
		}()
		go func() {
			bitcoinWallet.Run()
			log.Print("Wallet has stopped")
			close(walletStopped)
			wg.Done()
		}()
		go func() {
			eventBroker.Run()
			log.Print("Event broker has stopped")
			close(apiServerStopped)
			wg.Done()
		}()

		wg.Wait()
		log.Printf("Bitcoin processing has stopped")
	})
}
