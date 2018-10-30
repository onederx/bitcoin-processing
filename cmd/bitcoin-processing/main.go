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

		nodeapi.InitBTCRPC()

		go events.Start()
		go api.RunAPIServer(settings.GetString("api.http.address"))
		go wallet.Start()

		select {}
	})
}
