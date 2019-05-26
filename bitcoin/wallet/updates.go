package wallet

import (
	"log"
	"time"

	"github.com/btcsuite/btcutil"

	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/util"
)

var unknownAccountError = map[string]interface{}{"error": "account not found"}
var hotStorageMeta = map[string]interface{}{"kind": "input to hot storage"}

func getTransactionNotificationType(confirmations int64, tx *Transaction) events.EventType {
	switch tx.Direction {
	case IncomingDirection:
		if confirmations == 0 {
			return events.NewIncomingTxEvent
		}
		return events.IncomingTxConfirmedEvent
	case OutgoingDirection:
		if confirmations == 0 {
			return events.NewOutgoingTxEvent
		}
		return events.OutgoingTxConfirmedEvent
	default:
		panic("Unexpected tx direction " + tx.Direction.String())
	}
}

func (w *Wallet) getAccountMetainfo(tx *Transaction) (map[string]interface{}, error) {
	account, err := w.storage.GetAccountByAddress(tx.Address)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return unknownAccountError, nil
	}
	return account.Metainfo, nil
}

func (w *Wallet) notifyTransaction(tx *Transaction) error {
	confirmationsToNotify := util.Min64(tx.Confirmations, w.maxConfirmations)

	for i := tx.reportedConfirmations + 1; i <= confirmationsToNotify; i++ {
		eventType := getTransactionNotificationType(i, tx)

		// make a copy of tx here, otherwise it may get modified while
		// other goroutines process notification
		notification := *tx
		notification.Confirmations = i // Send confirmations sequentially

		if i == 0 {
			// in case this tx is already confirmed (fresh tx event was missed)
			// block hash will be set - make it empty so that tx in notification
			// looks like unconfirmed one
			notification.BlockHash = ""
		}
		w.setTxStatusByConfirmations(&notification)

		err := w.NotifyTransaction(eventType, notification)
		if err != nil {
			return err
		}
		err = w.storage.updateReportedConfirmations(tx, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Wallet) updateTxInfo(tx *Transaction) (bool, error) {
	var err error

	isHotStorageTx := tx.Address == w.hotWalletAddress
	if tx.Direction == IncomingDirection {
		if !isHotStorageTx {
			tx.Metainfo, err = w.getAccountMetainfo(tx)
			if err != nil {
				return false, err
			}
		} else {
			tx.Metainfo = hotStorageMeta
		}
	}

	oldStatus := tx.Status
	w.setTxStatusByConfirmations(tx)

	tx, err = w.storage.StoreTransaction(tx)
	if err != nil {
		return false, err
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
			tx.ID,
		)
	}
	if !isHotStorageTx && !tx.ColdStorage { // don't notify about internal txns
		err = w.notifyTransaction(tx)
	}

	return txInfoChanged, err
}

func (w *Wallet) checkForNewTransactions() {
	anyTxInfoChanged := false
	err := w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
		lastSeenBlock, err := currWallet.storage.GetLastSeenBlockHash()
		if err != nil {
			return err
		}
		lastTxData, err := currWallet.nodeAPI.ListTransactionsSinceBlock(lastSeenBlock)
		if err != nil {
			return err
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
			currentTxInfoChanged, err := currWallet.updateTxInfo(tx)
			if err != nil {
				return err
			}
			anyTxInfoChanged = anyTxInfoChanged || currentTxInfoChanged
		}
		return currWallet.storage.SetLastSeenBlockHash(lastTxData.LastBlock)
	})
	if err != nil {
		log.Printf("wallet: error: failed to process new bitcoin txns: %v", err)
		return
	}

	w.eventBroker.SendNotifications()

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
	anyTxInfoChanged := false

	err := w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
		transactionsToCheck, err := currWallet.storage.GetBroadcastedTransactionsWithLessConfirmations(
			w.maxConfirmations,
		)
		if err != nil {
			return err
		}

		for _, tx := range transactionsToCheck {
			fullTxInfo, err := currWallet.nodeAPI.GetTransaction(tx.Hash)
			if err != nil {
				return err
			}
			tx.updateFromFullTxInfo(fullTxInfo)
			currentTxInfoChanged, err := currWallet.updateTxInfo(tx)
			if err != nil {
				return err
			}
			anyTxInfoChanged = anyTxInfoChanged || currentTxInfoChanged
		}
		return nil
	})

	if err != nil {
		log.Printf(
			"wallet: error: failed to process updates on existing bitcoin "+
				"txns: %v", err)
		return
	}

	w.eventBroker.SendNotifications()

	if anyTxInfoChanged {
		w.updatePendingTxns()
	}
}

func (w *Wallet) checkForWalletUpdates() {
	w.checkForNewTransactions()
	w.checkForExistingTransactionUpdates()
}

func (w *Wallet) mainLoop() {
	pollInterval := time.Duration(w.settings.GetInt("bitcoin.poll_interval"))
	ticker := time.NewTicker(pollInterval * time.Millisecond).C
	for {
		select {
		case <-ticker:
		case <-w.externalTxNotifications:
		case withdrawRequest := <-w.withdrawQueue:
			withdrawRequest.result <- w.sendWithdrawal(withdrawRequest.tx, true)
			close(withdrawRequest.result)
		case cancelRequest := <-w.cancelQueue:
			cancelRequest.result <- w.cancelPendingTx(cancelRequest.id)
			close(cancelRequest.result)
		case confirmRequest := <-w.confirmQueue:
			confirmRequest.result <- w.confirmPendingTx(confirmRequest.id)
			close(confirmRequest.result)
		case holdRequest := <-w.holdQueue:
			holdRequest.result <- w.holdWithdrawalUntilConfirmed(holdRequest.tx)
			close(holdRequest.result)
		case <-w.pendingTxUpdateTrigger:
			w.updatePendingTxns()
		}
		w.checkForWalletUpdates()
	}
}

// TriggerWalletUpdate can be used to notify wallet that there
// are relevant tx updates and that it should get updates from Bitcoin node
// immediately (without it, updates are polled periodically).
func (w *Wallet) TriggerWalletUpdate() {
	select {
	case w.externalTxNotifications <- struct{}{}:
	default:
	}
}
