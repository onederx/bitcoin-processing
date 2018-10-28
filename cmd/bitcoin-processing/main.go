package main

import (
	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/settings"
)

func main() {
	settings.ReadSettingsAndRun(func() {

		nodeapi.InitBTCRPC()
		wallet.StartWatchingWalletUpdates()

		api.RunAPIServer(settings.GetString("api.http.address"))
	})
}
