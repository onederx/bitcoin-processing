package wallet

import (
	"errors"
	"log"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

// WithdrawRequest is a structure with parameters that can be set for new
// withdrawal. In order to make a withdraw, caller must initialize this
// structire and pass it to Withdraw method
// Fields ID, FeeType and Metainfo are optional
// Address can be optional for withdrawals to hot storage (because hot storage
// address can be set in config)
type WithdrawRequest struct {
	ID       uuid.UUID         `json:"id,omitempty"`
	Address  string            `json:"address,omitempty"`
	Amount   bitcoin.BTCAmount `json:"amount"`
	Fee      bitcoin.BTCAmount `json:"fee,omitempty"`
	FeeType  string            `json:"fee_type,omitempty"`
	Metainfo interface{}       `json:"metainfo"`
}

type internalTxRequest struct {
	tx     *Transaction
	result chan error
}

type internalWithdrawRequest internalTxRequest
type internalHoldRequest internalTxRequest

func logWithdrawRequest(request *WithdrawRequest, feeType bitcoin.FeeType) {
	log.Printf(
		"Got withdraw request with id %s, to address %s, "+
			"amount %s (%d satoshi) and fee %s (%d satoshi) (type %s)."+
			"Metainfo: %v",
		request.ID,
		request.Address,
		request.Amount, request.Amount,
		request.Fee, request.Fee,
		feeType,
		request.Metainfo,
	)
}

func isInsufficientFundsError(err error) bool {
	rpcError, ok := err.(*nodeapi.JSONRPCError)
	if !ok {
		return false
	}
	return rpcError.Message == "Insufficient funds"
}

func (w *Wallet) checkWithdrawLimits(request *WithdrawRequest, feeType bitcoin.FeeType) (needManualConfirmation bool, err error) {
	if request.Amount < w.minWithdraw {
		return false, errors.New(
			"Error: refusing to withdraw " + request.Amount.String() +
				" because it is less than min withdraw amount " +
				w.minWithdraw.String(),
		)
	}

	if feeType == bitcoin.PerKBRateFee && request.Fee < w.minFeePerKb {
		return false, errors.New(
			"Error: refusing to withdraw with fee " + request.Fee.String() +
				" because it is less than min withdraw fee " +
				w.minFeePerKb.String() + " for fee type " + feeType.String(),
		)
	}

	if feeType == bitcoin.FixedFee && request.Fee < w.minFeeFixed {
		return false, errors.New(
			"Error: refusing to withdraw with fee " + request.Fee.String() +
				" because it is less than min withdraw fee " +
				w.minFeeFixed.String() + " for fee type " + feeType.String(),
		)
	}

	if request.Amount > w.minWithdrawWithoutManualConfirmation {
		return true, nil
	}

	return false, nil
}

func (w *Wallet) sendWithdrawal(tx *Transaction, updatePending bool) error {
	var sendMoneyFunc func(string, bitcoin.BTCAmount, bitcoin.BTCAmount, bool) (string, error)

	switch tx.FeeType {
	case bitcoin.PerKBRateFee:
		sendMoneyFunc = w.nodeAPI.SendWithPerKBFee
	case bitcoin.FixedFee:
		sendMoneyFunc = w.nodeAPI.SendWithFixedFee
	default:
		return errors.New("Fee type not supported: " + tx.FeeType.String())
	}

	err := w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
		return currWallet.storage.LockWallet(map[string]interface{}{
			"operation": "withdraw",
			"tx":        tx,
		})
	})

	if err != nil {
		return err
	}

	txHash, err := sendMoneyFunc(
		tx.Address,
		tx.Amount,
		tx.Fee,
		true, // recipient pays tx fee
	)

	if err != nil {
		unlockErr := w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
			return currWallet.storage.ClearWallet()
		})
		if unlockErr != nil {
			// TODO: retry and more graceful crash
			log.Fatal(unlockErr)
		}
		if tx.ColdStorage || !isInsufficientFundsError(err) {
			return err
		}
		log.Printf("Not enough funds to send tx %v, marking as pending", tx)
		err = w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
			return currWallet.updatePendingTxStatus(tx, PendingTransaction)
		})
		if err != nil {
			return err
		}
	} else {
		tx.Status = NewTransaction
		tx.Hash = txHash

		log.Printf(
			"Successfully created and broadcasted outgoing tx (withdrawal) %v",
			tx,
		)

		err = w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
			_, err := currWallet.storage.StoreTransaction(tx)
			if err != nil {
				return err
			}
			if !tx.ColdStorage {
				err = currWallet.notifyTransaction(tx)

				if err != nil {
					return err
				}
			}
			return currWallet.storage.ClearWallet()
		})

		if err != nil {
			// TODO: retry and more graceful crash
			log.Fatal(err)
		}
	}

	w.eventBroker.SendNotifications()

	if updatePending {
		w.updatePendingTxns()
	}

	return nil
}

func (w *Wallet) sendWithdrawalViaWalletUpdater(tx *Transaction) error {
	// to prevent races, actual withdraw will be done in wallet updater
	// goroutine
	resultCh := make(chan error)
	w.withdrawQueue <- internalWithdrawRequest{
		tx:     tx,
		result: resultCh,
	}
	return <-resultCh
}

func (w *Wallet) holdWithdrawalUntilConfirmedViaWalletUpdater(tx *Transaction) error {
	// to prevent races, actual hold will be done in wallet updater
	// goroutine
	log.Printf(
		"Withdrawal %v has amount %s which is more than configured max "+
			"amount to be processed without manual confirmation. Holding it "+
			"until confirmed manually.",
		tx,
		tx.Amount,
	)
	resultCh := make(chan error)
	w.holdQueue <- internalHoldRequest{
		tx:     tx,
		result: resultCh,
	}
	return <-resultCh
}

func (w *Wallet) holdWithdrawalUntilConfirmed(tx *Transaction) error {
	err := w.MakeTransactIfAvailable(func(currWallet *Wallet) error {
		return currWallet.updatePendingTxStatus(
			tx,
			PendingManualConfirmationTransaction,
		)
	})
	if err != nil {
		return err
	}
	w.eventBroker.SendNotifications()
	return nil
}

// Withdraw makes a new withdrawal using parameters set in given WithdrawRequest
// This method makes several checks on request and then either rejects it or
// tries to perform a withdrawal.
// Withdrawal to hot wallet address are not allowed.
// Regular withdrawal may require manual confirmation if its amount exceeds a
// certain value ("wallet.min_withdraw_without_manual_confirmation" in config).
// There are restrictions on minimal withdrawal amount and fee value (set in
// config by "wallet.min_withdraw", "wallet.min_fee.per_kb",
// "wallet.min_fee.fixed")
// If withdrawal is allowed, but there is not enough money to send it, it
// becomes pending (it will receive status 'pending' which may be then changed
// to 'pending-cold-storage')
// In any case, actual withdrawal if performed in a wallet updater goroutine
// Argument toColdStorage tells whether this is a withdraw to cold storage - if
// so, address can be taken from config (intead of being set in request), also,
// withdrawals to cold storage never require manual confirmation and can't
// become pending (which means they fail immediately if there is not enough
// money to fund such withdrawal right now.)
func (w *Wallet) Withdraw(request *WithdrawRequest, toColdStorage bool) error {
	feeType, err := bitcoin.FeeTypeFromString(request.FeeType)
	if err != nil {
		return err
	}

	logWithdrawRequest(request, feeType)

	needManualConfirmation, err := w.checkWithdrawLimits(request, feeType)
	if err != nil {
		return err
	}

	if request.Address == "" {
		if !toColdStorage {
			return errors.New("Can't process withdraw: address is empty")
		}
		if w.coldWalletAddress == "" {
			return errors.New(
				"Withdraw to cold storage failed: address is not given in " +
					"request and not set in config",
			)
		}
		log.Printf(
			"Making transfer to cold storage address set in config: %s",
			w.coldWalletAddress,
		)
		request.Address = w.coldWalletAddress
	}

	if request.Address == w.hotWalletAddress {
		return errors.New(
			"Refusing to withdraw to hot wallet address: this operation " +
				"makes no sence because hot wallet address belongs to " +
				"wallet of bitcoin processing app",
		)
	}

	outgoingTx := &Transaction{
		ID:                    request.ID,
		Confirmations:         0,
		Address:               request.Address,
		Direction:             OutgoingDirection,
		Amount:                request.Amount,
		Metainfo:              request.Metainfo,
		Fee:                   request.Fee,
		FeeType:               feeType,
		ColdStorage:           toColdStorage,
		fresh:                 true,
		reportedConfirmations: -1,
	}

	if !needManualConfirmation || toColdStorage {
		// withdraw to cold storage does not need confirmation
		return w.sendWithdrawalViaWalletUpdater(outgoingTx)
	}
	return w.holdWithdrawalUntilConfirmedViaWalletUpdater(outgoingTx)
}
