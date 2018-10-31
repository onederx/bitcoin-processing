package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/onederx/bitcoin-processing/util"
	"log"
	"time"
)

type TransactionNotification struct {
	Transaction
	AccountMetainfo map[string]interface{} `json:"metainfo"`
}

var unknownAccountError = map[string]interface{}{
	"error": "account not found",
}

var maxConfirmations int64

func notifyIncomingTransaction(tx *Transaction, confirmationsToNotify int64) {
	var eventType events.EventType
	var accountMetainfo map[string]interface{}

	for i := tx.reportedConfirmations + 1; i <= confirmationsToNotify; i++ {
		if i == 0 {
			eventType = events.NewIncomingTxEvent
		} else {
			eventType = events.IncomingTxConfirmedEvent
		}
		account := storage.GetAccountByAddress(tx.Address)
		if account == nil {
			log.Printf(
				"Error: failed to match account by address %s "+
					"(transaction %s) for incoming payment",
				tx.Address,
				tx.Hash,
			)
			accountMetainfo = unknownAccountError
		} else {
			accountMetainfo = account.Metainfo
		}
		// make a copy of tx here, otherwise it may get modified while
		// other goroutines process notification
		notification := TransactionNotification{
			*tx,
			accountMetainfo,
		}
		notification.Confirmations = i // Send confirmations sequentially

		events.Notify(eventType, notification)
		storage.updateReportedConfirmations(tx, i)
	}

}

func notifyTransaction(tx *Transaction) {
	confirmationsToNotify := util.Min64(tx.Confirmations, maxConfirmations)

	if tx.Direction == IncomingDirection {
		notifyIncomingTransaction(tx, confirmationsToNotify)
	}
}

func updateTxInfo(tx *Transaction) {
	tx = storage.StoreTransaction(tx)
	notifyTransaction(tx)
}

func checkForNewTransactions() {
	lastSeenBlock := storage.GetLastSeenBlockHash()
	lastTxData, err := nodeapi.ListTransactionsSinceBlock(lastSeenBlock)
	if err != nil {
		log.Print("Error: Checking for wallet updates failed: ", err)
		return
	}

	if len(lastTxData.Transactions) > 0 || lastTxData.LastBlock != lastSeenBlock {
		log.Printf(
			"Got %d transactions from node. Last block hash is %s",
			len(lastTxData.Transactions),
			lastTxData.LastBlock,
		)
	}
	for _, btcNodeTransaction := range lastTxData.Transactions {
		tx := newTransaction(&btcNodeTransaction)
		updateTxInfo(tx)
	}
	storage.SetLastSeenBlockHash(lastTxData.LastBlock)
}

func checkForExistingTransactionUpdates() {
	transactionsToCheck := storage.GetTransactionsWithLessConfirmations(maxConfirmations)

	for _, tx := range transactionsToCheck {
		fullTxInfo, err := nodeapi.GetTransaction(tx.Hash)

		if err != nil {
			log.Printf(
				"Error: could not get tx %s from node for update",
				tx.Hash,
			)
			continue
		}
		tx.updateFromFullTxInfo(fullTxInfo)
		updateTxInfo(tx)
	}
}

func checkForWalletUpdates() {
	checkForNewTransactions()
	checkForExistingTransactionUpdates()
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
	maxConfirmations = int64(settings.GetInt("transaction.max-confirmations"))
	pollWalletUpdates()
}
