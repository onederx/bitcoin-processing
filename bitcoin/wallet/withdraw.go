package wallet

import (
	"errors"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/satori/go.uuid"
	"log"
)

type WithdrawRequest struct {
	Id       uuid.UUID             `json:"id"`
	Address  string                `json:"address"`
	Amount   bitcoin.BitcoinAmount `json:"amount"`
	Fee      bitcoin.BitcoinAmount `json:"fee"`
	FeeType  string                `json:"fee_type"`
	Metainfo interface{}           `json:"metainfo"`
}

type internalWithdrawRequest struct {
	tx     *Transaction
	result chan error
}

func logWithdrawRequest(request *WithdrawRequest, feeType bitcoin.FeeType) {
	log.Printf(
		"Got withdraw request with id %s, to address %s, "+
			"satoshi amount %d and fee %d (type %s). Metainfo: %v",
		request.Id,
		request.Address,
		request.Amount,
		request.Fee,
		feeType,
		request.Metainfo,
	)
}

func isInsufficientFundsError(err error) bool {
	rpcError, ok := err.(*nodeapi.JsonRPCError)

	if ok {
		return rpcError.Message == "Insufficient funds"
	}
	return false
}

func (w *Wallet) checkWithdrawLimits(request *WithdrawRequest, feeType bitcoin.FeeType) error {
	if request.Amount < w.minWithdraw {
		return errors.New(
			"Error: refusing to withdraw " + request.Amount.String() +
				" because it is less than min withdraw amount " +
				w.minWithdraw.String(),
		)
	}
	if feeType == bitcoin.PerKBRateFee && request.Fee < w.minFeePerKb {
		return errors.New(
			"Error: refusing to withdraw with fee " + request.Fee.String() +
				" because it is less than min withdraw fee " +
				w.minFeePerKb.String() + " for fee type " + feeType.String(),
		)
	}
	if feeType == bitcoin.FixedFee && request.Fee < w.minFeeFixed {
		return errors.New(
			"Error: refusing to withdraw with fee " + request.Fee.String() +
				" because it is less than min withdraw fee " +
				w.minFeeFixed.String() + " for fee type " + feeType.String(),
		)
	}
	return nil
}

func (w *Wallet) sendWithdrawal(tx *Transaction, updatePending bool) error {
	var sendMoneyFunc func(string, bitcoin.BitcoinAmount, bitcoin.BitcoinAmount, bool) (string, error)

	switch tx.FeeType {
	case bitcoin.PerKBRateFee:
		sendMoneyFunc = w.nodeAPI.SendWithPerKBFee
	case bitcoin.FixedFee:
		sendMoneyFunc = w.nodeAPI.SendWithFixedFee
	default:
		return errors.New("Fee type not supported: " + tx.FeeType.String())
	}

	txHash, err := sendMoneyFunc(
		tx.Address,
		tx.Amount,
		tx.Fee,
		true, // recipient pays tx fee
	)

	if err != nil {
		if tx.ColdStorage || !isInsufficientFundsError(err) {
			return err
		}
		log.Printf("Not enough funds to send tx %v, marking as pending", tx)
		err = w.updatePendingTxStatus(tx, PendingTransaction)
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

		_, err = w.storage.StoreTransaction(tx)
		if err != nil {
			return err
		}
		if !tx.ColdStorage {
			w.notifyTransaction(tx)
		}
	}

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

func (w *Wallet) Withdraw(request *WithdrawRequest, toColdStorage bool) error {
	feeType, err := bitcoin.FeeTypeFromString(request.FeeType)

	if err != nil {
		return err
	}

	logWithdrawRequest(request, feeType)

	err = w.checkWithdrawLimits(request, feeType)

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
		Id:                    request.Id,
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

	return w.sendWithdrawalViaWalletUpdater(outgoingTx)
}
