package wallet

func (w *Wallet) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error) {
	return w.storage.GetTransactionsWithFilter(directionFilter, statusFilter)
}
