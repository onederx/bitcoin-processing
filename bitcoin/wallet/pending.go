package wallet

import (
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/events"
)

type internalCancelRequest struct {
	id     uuid.UUID
	result chan error
}

func (w *Wallet) updatePendingTxStatus(tx *Transaction, status TransactionStatus) error {
	if status == tx.Status {
		return nil
	}

	tx.Status = status
	if _, err := w.storage.StoreTransaction(tx); err != nil {
		return err
	}

	var eventType events.EventType
	if status == CancelledTransaction {
		eventType = events.PendingTxCancelledEvent
	} else {
		eventType = events.PendingStatusUpdatedEvent
	}
	w.NotifyTransaction(eventType, *tx)

	return nil
}

func (w *Wallet) updatePendingTxStatusOrLogError(tx *Transaction, status TransactionStatus) {
	if err := w.updatePendingTxStatus(tx, status); err != nil {
		log.Printf(
			"Error: failed to store updated pending tx status: %s",
			err,
		)
	}
}

func (w *Wallet) updatePendingTxns() {
	pendingTxns, err := w.storage.GetPendingTransactions()
	if err != nil {
		log.Printf("Error: failed to get pending txns for update %s", err)
		return
	}
	sort.Slice(pendingTxns, func(i, j int) bool {
		return pendingTxns[i].Amount < pendingTxns[j].Amount
	})

	confBal, unconfBal, err := w.nodeAPI.GetConfirmedAndUnconfirmedBalance()
	if err != nil {
		log.Printf("Error: failed to get wallet balance %s", err)
		return
	}
	availableBalance := int64(confBal)

	var amount int64
	exceedingTx := -1
	for i, tx := range pendingTxns {
		amount = int64(tx.Amount)
		if availableBalance-amount >= 0 {
			availableBalance -= amount
			log.Printf("There is now enough money to send tx %v, resending", tx)
			err = w.sendWithdrawal(tx, false)
			if err != nil {
				log.Printf("Error re-sending pending tx: %s", err)
			}
		} else {
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
		err := w.storage.SetMoneyRequiredFromColdStorage(uint64(-availableBalance))
		if err != nil {
			log.Printf("Error saving amount required from cold storage %s", err)
		}
	} else if w.storage.GetMoneyRequiredFromColdStorage() > 0 {
		w.storage.SetMoneyRequiredFromColdStorage(0)
	}
}

func (w *Wallet) cancelPendingTx(id uuid.UUID) error {
	tx, err := w.storage.GetTransactionById(id)
	if err != nil {
		return err
	}

	switch tx.Status {
	case PendingTransaction:
	case PendingColdStorageTransaction:
	case PendingManualConfirmationTransaction:
	default:
		return errors.New("Transaction is not pending")
	}

	err = w.updatePendingTxStatus(tx, CancelledTransaction)
	if err != nil {
		return err
	}

	w.updatePendingTxns()
	return nil
}

func (w *Wallet) CancelPendingTx(id uuid.UUID) error {
	// to prevent races, actual cancellation will be done in wallet updater
	// goroutine
	resultCh := make(chan error)
	w.cancelQueue <- internalCancelRequest{
		id:     id,
		result: resultCh,
	}
	return <-resultCh
}

func (w *Wallet) ConfirmPendingTransaction(id uuid.UUID) error {
	tx, err := w.storage.GetTransactionById(id)
	if err != nil {
		return err
	}

	if tx.Status != PendingManualConfirmationTransaction {
		return fmt.Errorf(
			"Tx %s is not pending manual confirmation. Its status is %s",
			id,
			tx.Status,
		)
	}

	return w.sendWithdrawal(tx, true)
}
