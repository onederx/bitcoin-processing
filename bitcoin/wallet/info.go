package wallet

func (w *Wallet) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error) {
	return w.storage.GetTransactionsWithFilter(directionFilter, statusFilter)
}

func (w *Wallet) GetBalance() (uint64, uint64, error) {
	conf, unconf, err := w.nodeAPI.GetConfirmedAndUnconfirmedBalance()
	if err != nil {
		return 0, 0, err
	}
	return conf, conf + unconf, nil
}
