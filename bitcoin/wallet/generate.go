package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

type Account struct {
	Address  string
	Metainfo map[string]interface{}
}

func GenerateNewAddress() string {
	address, err := nodeapi.CreateNewAddress()
	if err != nil {
		panic(err)
	}
	addressStr := address.EncodeAddress()
	return addressStr
}
