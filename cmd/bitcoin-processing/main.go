package main

import (
	"log"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/settings"
)

func main() {
	settings.ReadSettingsAndRun(func() {

		nodeapi.InitBTCRPC()

		log.Printf("Using tx callback %#v", settings.GetURL("tx-callback"))

		api.RunAPIServer(settings.GetString("api.http.address"))
	})
}
