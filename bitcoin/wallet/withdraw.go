package wallet

import (
	"errors"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/satori/go.uuid"
	"log"
	"strconv"
)

type WithdrawRequest struct {
	Id       uuid.UUID
	Address  string
	Amount   uint64
	Fee      uint64
	FeeType  string `json:"fee_type"`
	Metainfo interface{}
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

func (w *Wallet) checkWithdrawLimits(request *WithdrawRequest, feeType bitcoin.FeeType) error {
	if request.Amount < w.minWithdraw {
		return errors.New(
			"Error: refusing to withdraw " +
				strconv.FormatUint(request.Amount, 10) +
				" because it is less than min withdraw amount " +
				strconv.FormatUint(w.minWithdraw, 10),
		)
	}
	if feeType == bitcoin.PerKBRateFee && request.Fee < w.minFeePerKb {
		return errors.New(
			"Error: refusing to withdraw with fee" +
				strconv.FormatUint(request.Fee, 10) +
				" because it is less than min withdraw fee " +
				strconv.FormatUint(w.minFeePerKb, 10) +
				" for fee type " + feeType.String(),
		)
	}
	if feeType == bitcoin.FixedFee && request.Fee < w.minFeeFixed {
		return errors.New(
			"Error: refusing to withdraw with fee" +
				strconv.FormatUint(request.Fee, 10) +
				" because it is less than min withdraw fee " +
				strconv.FormatUint(w.minFeeFixed, 10) +
				" for fee type " + feeType.String(),
		)
	}
	return nil
}

func (w *Wallet) Withdraw(request *WithdrawRequest) error {
	var sendMoneyFunc func(string, uint64, uint64, bool) (string, error)
	feeType, err := bitcoin.FeeTypeFromString(request.FeeType)

	if err != nil {
		return err
	}

	logWithdrawRequest(request, feeType)

	err = w.checkWithdrawLimits(request, feeType)

	if err != nil {
		return err
	}

	switch feeType {
	case bitcoin.PerKBRateFee:
		sendMoneyFunc = w.nodeAPI.SendWithPerKBFee
	case bitcoin.FixedFee:
		sendMoneyFunc = w.nodeAPI.SendWithFixedFee
	default:
		return errors.New("Fee type not supported: " + request.FeeType)
	}

	txHash, err := sendMoneyFunc(
		request.Address,
		request.Amount,
		request.Fee,
		true, // recipient pays tx fee
	)

	if err != nil {
		return err
	}

	outgoingTx := &Transaction{
		Id:                    request.Id,
		Hash:                  txHash,
		Confirmations:         0,
		Address:               request.Address,
		Direction:             OutgoingDirection,
		Status:                NewTransaction,
		Amount:                request.Amount,
		Metainfo:              request.Metainfo,
		reportedConfirmations: -1,
	}

	w.notifyTransaction(outgoingTx)

	_, err = w.storage.StoreTransaction(outgoingTx)

	if err != nil {
		return err
	}

	return nil
}
