package wallet

type Account struct {
	Address  string                 `json:"address"`
	Metainfo map[string]interface{} `json:"metainfo"`
}

func (w *Wallet) generateNewAddress() string {
	address, err := w.nodeAPI.CreateNewAddress()
	if err != nil {
		panic(err)
	}
	addressStr := address.EncodeAddress()
	return addressStr
}

func (w *Wallet) CreateAccount(metainfo map[string]interface{}) *Account {
	account := &Account{
		Address:  w.generateNewAddress(),
		Metainfo: metainfo,
	}
	w.storage.StoreAccount(account)
	return account
}
