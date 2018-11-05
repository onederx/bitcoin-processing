package wallet

import (
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

var unknownAccountError = map[string]interface{}{"error": "account not found"}

func (w *Wallet) notifyIncomingTransaction(tx *Transaction, confirmationsToNotify int64) {
	var eventType events.EventType
	var accountMetainfo map[string]interface{}

	for i := tx.reportedConfirmations + 1; i <= confirmationsToNotify; i++ {
		if i == 0 {
			eventType = events.NewIncomingTxEvent
		} else {
			eventType = events.IncomingTxConfirmedEvent
		}
		account, err := w.storage.GetAccountByAddress(tx.Address)
		if err != nil {
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

		w.eventBroker.Notify(eventType, notification)
		err = w.storage.updateReportedConfirmations(tx, i)
		if err != nil {
			log.Printf(
				"Error: failed to update count of reported transaction "+
					"confirmations in storage: %s",
				err,
			)
			return
		}
	}

}

func (w *Wallet) notifyTransaction(tx *Transaction) {
	confirmationsToNotify := util.Min64(tx.Confirmations, w.maxConfirmations)

	if tx.Direction == IncomingDirection {
		w.notifyIncomingTransaction(tx, confirmationsToNotify)
	}
}

func (w *Wallet) updateTxInfo(tx *Transaction) {
	tx, err := w.storage.StoreTransaction(tx)
	if err != nil {
		log.Printf("Error: failed to store tx data in database: %s", err)
		return
	}
	w.notifyTransaction(tx)
}

func (w *Wallet) checkForNewTransactions() {
	lastSeenBlock := w.storage.GetLastSeenBlockHash()
	lastTxData, err := w.nodeAPI.ListTransactionsSinceBlock(lastSeenBlock)
	if err != nil {
		log.Print("Error: Checking for wallet updates failed: ", err)
		return
	}

	if lastTxData.LastBlock != lastSeenBlock {
		log.Printf(
			"Got %d transactions from node. Last block hash is %s",
			len(lastTxData.Transactions),
			lastTxData.LastBlock,
		)
	}
	for _, btcNodeTransaction := range lastTxData.Transactions {
		tx := newTransaction(&btcNodeTransaction)
		w.updateTxInfo(tx)
	}
	err = w.storage.SetLastSeenBlockHash(lastTxData.LastBlock)
	if err != nil {
		log.Printf(
			"Error: failed to update last seen block hash in db: %s",
			err,
		)
	}
}

func (w *Wallet) checkForExistingTransactionUpdates() {
	transactionsToCheck, err := w.storage.GetTransactionsWithLessConfirmations(
		w.maxConfirmations,
	)

	if err != nil {
		log.Printf(
			"Error: failed to fetch transactions from storage for update: %s",
			err,
		)
		return
	}

	for _, tx := range transactionsToCheck {
		fullTxInfo, err := w.nodeAPI.GetTransaction(tx.Hash)

		if err != nil {
			log.Printf(
				"Error: could not get tx %s from node for update",
				tx.Hash,
			)
			continue
		}
		tx.updateFromFullTxInfo(fullTxInfo)
		w.updateTxInfo(tx)
	}
}

func (w *Wallet) checkForWalletUpdates() {
	w.checkForNewTransactions()
	w.checkForExistingTransactionUpdates()
}

func (w *Wallet) pollWalletUpdates() {
	pollInterval := time.Duration(settings.GetInt("bitcoin.poll-interval"))
	ticker := time.NewTicker(pollInterval * time.Millisecond).C
	for {
		select {
		case <-ticker:
		case <-w.eventBroker.ExternalTxNotifications:
		}
		w.checkForWalletUpdates()
	}
}

func (w *Wallet) startWatchingWalletUpdates() {
	w.pollWalletUpdates()
}
