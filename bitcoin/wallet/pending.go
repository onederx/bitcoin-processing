package wallet

import (
	"log"
	"sort"

	"github.com/onederx/bitcoin-processing/events"
)

func (w *Wallet) updatePendingTxStatus(tx *Transaction, status TransactionStatus) error {
	if status == tx.Status {
		return nil
	}
	tx.Status = status
	_, err := w.storage.StoreTransaction(tx)
	if err != nil {
		return err
	}
	w.eventBroker.Notify(events.PendingStatusUpdatedEvent, *tx)
	return nil
}

func (w *Wallet) updatePendingTxStatusOrLogError(tx *Transaction, status TransactionStatus) {
	err := w.updatePendingTxStatus(tx, PendingTransaction)
	if err != nil {
		log.Printf(
			"Error: failed to store updated pending tx status: %s",
			err,
		)
	}
}

func (w *Wallet) updatePendingTxns() {
	var amount int64
	log.Printf("Updating pending txns")
	pendingTxns, err := w.storage.GetPendingTransactions()
	if err != nil {
		log.Printf("Error: failed to get pending txns for update %s", err)
		return
	}
	sort.Slice(pendingTxns, func(i, j int) bool {
		return pendingTxns[i].Amount < pendingTxns[j].Amount
	})
	confBal, unconfBal, err := w.nodeAPI.GetConfirmedAndUnconfirmedBalance()
	log.Printf("Balances are %d %d", confBal, unconfBal)
	if err != nil {
		log.Printf("Error: failed to get wallet balance %s", err)
		return
	}
	availableBalance := int64(confBal)

	exceedingTx := -1

	for i, tx := range pendingTxns {
		amount = int64(tx.Amount)
		if availableBalance-amount >= 0 {
			availableBalance -= amount
			log.Printf("Enough money to send tx %v, balance now is %d", tx, availableBalance)
			err = w.sendWithdrawal(tx, false)
			if err != nil {
				log.Printf("Error re-sending pending tx: %s", err)
			}
		} else {
			log.Printf("Not enough balance to send tx %v", tx)
			exceedingTx = i
			break
		}
	}
	if exceedingTx != -1 && unconfBal > 0 {
		// we did not have enough money to fund all pending txns, but we have
		// some unconfirmed balance, maybe we'll be able to fund some pending
		// txns when this balance is confirmed
		pendingTxns = pendingTxns[exceedingTx:]
		exceedingTx = -1
		availableBalance += int64(unconfBal)
		for i, tx := range pendingTxns {
			amount = int64(tx.Amount)
			if availableBalance-amount >= 0 {
				availableBalance -= amount
				w.updatePendingTxStatusOrLogError(tx, PendingTransaction)
			} else {
				exceedingTx = i
				break
			}
		}
	}

	if exceedingTx != -1 {
		for _, tx := range pendingTxns[exceedingTx:] {
			availableBalance -= int64(tx.Amount)
			w.updatePendingTxStatusOrLogError(tx, PendingColdStorageTransaction)
		}
		err := w.storage.SetMoneyRequiredFromColdStorage(
			uint64(-availableBalance),
		)
		if err != nil {
			log.Printf("Error saving amount required from cold storage %s", err)
		}
	} else if w.storage.GetMoneyRequiredFromColdStorage() > 0 {
		w.storage.SetMoneyRequiredFromColdStorage(0)
	}
}
