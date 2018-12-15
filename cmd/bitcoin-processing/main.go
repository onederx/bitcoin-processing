package main

import (
	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

func main() {
	settings.ReadSettingsAndRun(func() {

		nodeAPI := nodeapi.NewNodeAPI()
		eventBroker := events.NewEventBroker()
		bitcoinWallet := wallet.NewWallet(nodeAPI, eventBroker)
		apiServer := api.NewServer(
			settings.GetString("api.http.address"),
			bitcoinWallet,
			eventBroker,
		)

		go apiServer.Run()
		go bitcoinWallet.Run()
		go eventBroker.Run()

		select {}
	})
}
