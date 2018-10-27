package main

import (
	"log"

	"github.com/onederx/bitcoin-processing/config"
)

func main() {
	config.ReadSettingsAndRun(func() {
		log.Printf("Using tx callback %s", config.GetString("tx-callback"))
	})
}
