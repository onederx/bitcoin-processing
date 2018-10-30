package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

type Account struct {
	Address  string                 `json:"address"`
	Metainfo map[string]interface{} `json:"metainfo"`
}

func generateNewAddress() string {
	address, err := nodeapi.CreateNewAddress()
	if err != nil {
		panic(err)
	}
	addressStr := address.EncodeAddress()
	return addressStr
}

func CreateAccount(metainfo map[string]interface{}) *Account {
	account := &Account{
		Address:  generateNewAddress(),
		Metainfo: metainfo,
	}
	storage.StoreAccount(account)
	return account
}
