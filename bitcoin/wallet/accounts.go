package wallet

// Account describes user account. It consists of bitcoin address and metainfo
// supplied when account was created
type Account struct {
	Address  string                 `json:"address"`
	Metainfo map[string]interface{} `json:"metainfo"`
}

func (w *Wallet) generateNewAddress() (string, error) {
	address, err := w.nodeAPI.CreateNewAddress()
	if err != nil {
		return "", err
	}
	addressStr := address.EncodeAddress()
	return addressStr, nil
}

// CreateAccount creates new Account: generates new bitcoin address and stores
// it in DB along with given assosiated metainfo
func (w *Wallet) CreateAccount(metainfo map[string]interface{}) (*Account, error) {
	address, err := w.generateNewAddress()
	if err != nil {
		return nil, err
	}
	account := &Account{
		Address:  address,
		Metainfo: metainfo,
	}
	err = w.storage.StoreAccount(account)
	if err != nil {
		return nil, err
	}
	return account, nil
}
