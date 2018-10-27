package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
)

type Account struct {
	Address string
}

func GenerateNewAddress(account Account) string {
	address, err := nodeapi.CreateNewAddress()
	if err != nil {
		panic(err)
	}
	return address.EncodeAddress()
}
