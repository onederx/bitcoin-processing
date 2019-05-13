package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin"
)

// GetTransactionsWithFilter fetches transactions from storage filtered by
// status and direction. Empty filter string means do not filter, nonempty
// string means only transactions with equal value of corresponding parameter
// will be included
func (w *Wallet) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error) {
	return w.storage.GetTransactionsWithFilter(directionFilter, statusFilter)
}

// GetBalance returns current wallet balance. More precisely, it returns two BTC
// amounts: current confirmed balance (which is a balance that can already be
// spent, provided by already mined transactions) and balance including
// unconfirmed (which is a total balance that will be available for spending
// when all unconfirmed incoming txns will be confirmed)
func (w *Wallet) GetBalance() (bitcoin.BTCAmount, bitcoin.BTCAmount, error) {
	conf, unconf, err := w.nodeAPI.GetConfirmedAndUnconfirmedBalance()
	if err != nil {
		return 0, 0, err
	}
	return bitcoin.BTCAmount(conf), bitcoin.BTCAmount(conf + unconf), nil
}

// GetMoneyRequiredFromColdStorage returns amount of BTC that should be
// transfered from cold storage in order to cover all pending outgoing
// withdrawals. In case current wallet balance (including unconfirmed) is
// enough to cover all current withdrawals, this value will be zero. Otherwise,
// withdrawals that processing won't be able to cover with balance it has will
// receive status 'pending-cold-storage' and GetMoneyRequiredFromColdStorage()
// will return how much more money wallet needs to pay for them all
func (w *Wallet) GetMoneyRequiredFromColdStorage() (bitcoin.BTCAmount, error) {
	amount, err := w.storage.GetMoneyRequiredFromColdStorage()

	if err != nil {
		return 0, err
	}

	return bitcoin.BTCAmount(amount), nil
}
