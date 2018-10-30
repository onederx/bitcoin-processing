package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
	"log"
	"time"
)

func checkForWalletUpdates() {
	lastSeenBlock := storage.GetLastSeenBlockHash()
	log.Print("Last seen block hash ", lastSeenBlock)
	lastTxData, err := nodeapi.ListTransactionsSinceBlock(lastSeenBlock)
	if err != nil {
		log.Print("Checking for wallet updates failed:", err)
		return
	}

	if len(lastTxData.Transactions) > 0 || lastTxData.LastBlock != lastSeenBlock {
		log.Printf(
			"Got %d transactions from node. Last block hash is %s",
			len(lastTxData.Transactions),
			lastTxData.LastBlock,
		)
	}
	for _, transaction := range lastTxData.Transactions {
		log.Printf("Process tx %#v", transaction)

		storage.StoreTransaction(&Transaction{
			Hash:          transaction.TxID,
			BlockHash:     transaction.BlockHash,
			Confirmations: transaction.Confirmations,
			Address:       transaction.Address,
		})
	}
	storage.SetLastSeenBlockHash(lastTxData.LastBlock)
}

func pollWalletUpdates() {
	pollInterval := time.Duration(settings.GetInt("bitcoin.poll-interval"))
	ticker := time.NewTicker(pollInterval * time.Millisecond).C
	for {
		select {
		case <-ticker:
		case <-events.ExternalTxNotifications:
		}
		checkForWalletUpdates()
	}
}

func startWatchingWalletUpdates() {
	pollWalletUpdates()
}
