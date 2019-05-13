package main

import (
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
		)

		go apiServer.Run()
		go bitcoinWallet.Run()
		go eventBroker.Run()

		select {}
	})
}
