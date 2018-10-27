package main

import (
	"log"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/config"
)

func main() {
	config.ReadSettingsAndRun(func() {

		nodeapi.InitBTCRPC()

		log.Printf("Using tx callback %#v", config.GetURL("tx-callback"))

		api.RunAPIServer(config.GetString("api.http.address"))
	})
}
