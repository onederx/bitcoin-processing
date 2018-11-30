package wallet

import (
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/onederx/bitcoin-processing/util"
	"github.com/btcsuite/btcutil"
	"log"
	"time"
)

var unknownAccountError = map[string]interface{}{"error": "account not found"}
var hotStorageMeta = map[string]interface{}{"kind": "input to hot storage"}

func getTransactionNotificationType(confirmations int64, tx *Transaction) events.EventType {
	switch tx.Direction {
	case IncomingDirection:
		if confirmations == 0 {
			return events.NewIncomingTxEvent
		} else {
			return events.IncomingTxConfirmedEvent
		}
	case OutgoingDirection:
		if confirmations == 0 {
			return events.NewOutgoingTxEvent
		} else {
			return events.OutgoingTxConfirmedEvent
		}
	default:
		panic("Unexpected tx direction " + tx.Direction.String())
	}
}

func (w *Wallet) getAccountMetainfo(tx *Transaction) map[string]interface{} {
	account, err := w.storage.GetAccountByAddress(tx.Address)
	if err != nil {
		log.Printf(
			"Error: failed to match account by address %s "+
				"(transaction %s) for incoming payment",
			tx.Address,
			tx.Hash,
		)
		return unknownAccountError
	} else {
		return account.Metainfo
	}
}

func (w *Wallet) notifyTransaction(tx *Transaction) {
	confirmationsToNotify := util.Min64(tx.Confirmations, w.maxConfirmations)

	for i := tx.reportedConfirmations + 1; i <= confirmationsToNotify; i++ {
		eventType := getTransactionNotificationType(i, tx)

		// make a copy of tx here, otherwise it may get modified while
		// other goroutines process notification
		notification := *tx
		notification.Confirmations = i // Send confirmations sequentially
		w.setTxStatusByConfirmations(&notification)

		w.NotifyTransaction(eventType, notification)
		err := w.storage.updateReportedConfirmations(tx, i)
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

func (w *Wallet) updateTxInfo(tx *Transaction) bool {
	isHotStorageTx := tx.Address == w.hotWalletAddress
	if tx.Direction == IncomingDirection {
		if !isHotStorageTx {
			tx.Metainfo = w.getAccountMetainfo(tx)
		} else {
			tx.Metainfo = hotStorageMeta
		}
	}
	oldStatus := tx.Status
	w.setTxStatusByConfirmations(tx)
	tx, err := w.storage.StoreTransaction(tx)
	if err != nil {
		log.Printf("Error: failed to store tx data in database: %s", err)
		return false
	}
	txInfoChanged := tx.fresh || (oldStatus != tx.Status)
	if tx.fresh {
		log.Printf("New tx %s", tx.Hash)
	}
	if isHotStorageTx && tx.fresh {
		log.Printf(
			"Got transfer to hot wallet: %d satoshi (%s) tx %s (%s)",
			tx.Amount,
			btcutil.Amount(tx.Amount).String(),
			tx.Hash,
			tx.Id,
		)
	}
	if !isHotStorageTx && !tx.ColdStorage { // don't notify about internal txns
		w.notifyTransaction(tx)
	}
	return txInfoChanged
}

func (w *Wallet) checkForNewTransactions() {
	lastSeenBlock := w.storage.GetLastSeenBlockHash()
	lastTxData, err := w.nodeAPI.ListTransactionsSinceBlock(lastSeenBlock)
	anyTxInfoChanged := false
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
		currentTxInfoChanged := w.updateTxInfo(tx)
		anyTxInfoChanged = anyTxInfoChanged || currentTxInfoChanged
	}
	err = w.storage.SetLastSeenBlockHash(lastTxData.LastBlock)
	if err != nil {
		log.Printf(
			"Error: failed to update last seen block hash in db: %s",
			err,
		)
	}
	if anyTxInfoChanged {
		w.updatePendingTxns()
	}
}

func (w *Wallet) setTxStatusByConfirmations(tx *Transaction) {
	switch {
	case tx.Status != NewTransaction && tx.Status != ConfirmedTransaction && tx.Status != FullyConfirmedTransaction:
		// only "new" and "confirmed" statuses can be changed based on
		// number of confirmations ("new" can become "confirmed", "confirmed"
		// can become "fully-confirmed"). We also allow "fully-confirmed" to
		// changed because statuses should be sent consistently, so, we want to
		// be able to set status back to "new" or "confirmed" based on number of
		// confirmations to report these statuses to client before reporting
		// actual status "fully-confirmed"
		return
	case tx.Confirmations <= 0:
		tx.Status = NewTransaction
	case tx.Confirmations > 0 && tx.Confirmations < w.maxConfirmations:
		tx.Status = ConfirmedTransaction
	case tx.Confirmations >= w.maxConfirmations:
		tx.Status = FullyConfirmedTransaction
	}
}

func (w *Wallet) checkForExistingTransactionUpdates() {
	transactionsToCheck, err := w.storage.GetBroadcastedTransactionsWithLessConfirmations(
		w.maxConfirmations,
	)
	anyTxInfoChanged := false

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
		currentTxInfoChanged := w.updateTxInfo(tx)
		anyTxInfoChanged = anyTxInfoChanged || currentTxInfoChanged
	}
	if anyTxInfoChanged {
		w.updatePendingTxns()
	}
}

func (w *Wallet) checkForWalletUpdates() {
	w.checkForNewTransactions()
	w.checkForExistingTransactionUpdates()
}

func (w *Wallet) pollWalletUpdates() {
	pollInterval := time.Duration(settings.GetInt("bitcoin.poll_interval"))
	ticker := time.NewTicker(pollInterval * time.Millisecond).C
	for {
		select {
		case <-ticker:
		case <-w.eventBroker.ExternalTxNotifications:
		case withdrawRequest := <-w.withdrawQueue:
			withdrawRequest.result <- w.sendWithdrawal(withdrawRequest.tx, true)
			close(withdrawRequest.result)
		case cancelRequest := <-w.cancelQueue:
			cancelRequest.result <- w.cancelPendingTx(cancelRequest.id)
			close(cancelRequest.result)
		}
		w.checkForWalletUpdates()
	}
}

func (w *Wallet) startWatchingWalletUpdates() {
	w.pollWalletUpdates()
}
