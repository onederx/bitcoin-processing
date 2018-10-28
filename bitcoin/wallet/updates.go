package wallet

import (
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
	"log"
	"time"
)

func CheckForWalletUpdates() {
	log.Printf("Should now check bitcoin node for wallet updates")
}

func pollWalletUpdates() {
	pollInterval := time.Duration(settings.GetInt("bitcoin.poll-interval"))
	ticker := time.NewTicker(pollInterval * time.Millisecond).C
	for {
		select {
		case <-ticker:
		case <-events.ExternalTxNotifications:
		}
		CheckForWalletUpdates()
	}
}

func startWatchingWalletUpdates() {
	go pollWalletUpdates()
}
