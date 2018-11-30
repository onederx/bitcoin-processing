package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin"
)

func (w *Wallet) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error) {
	return w.storage.GetTransactionsWithFilter(directionFilter, statusFilter)
}

func (w *Wallet) GetBalance() (bitcoin.BitcoinAmount, bitcoin.BitcoinAmount, error) {
	conf, unconf, err := w.nodeAPI.GetConfirmedAndUnconfirmedBalance()
	if err != nil {
		return 0, 0, err
	}
	return bitcoin.BitcoinAmount(conf), bitcoin.BitcoinAmount(conf + unconf), nil
}

func (w *Wallet) GetMoneyRequiredFromColdStorage() bitcoin.BitcoinAmount {
	return bitcoin.BitcoinAmount(w.storage.GetMoneyRequiredFromColdStorage())
}
