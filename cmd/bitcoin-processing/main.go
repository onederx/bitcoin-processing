package main

import (
	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

func main() {
	settings.ReadSettingsAndRun(func(loadedSettings settings.Settings) {
		nodeAPI := nodeapi.NewNodeAPI(loadedSettings)
		storageType := loadedSettings.GetStringMandatory("storage.type")
		eventBroker := events.NewEventBroker(
			loadedSettings,
			events.NewEventStorage(storageType, loadedSettings),
		)
		bitcoinWallet := wallet.NewWallet(
			loadedSettings,
			nodeAPI,
			eventBroker,
			wallet.NewStorage(storageType, loadedSettings),
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
