package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
)

type Account struct {
	Address string
}

func GenerateNewAddress(account Account) string {
	address, err := nodeapi.CreateNewAddress()
	if err != nil {
		panic(err)
	}
	addressStr := address.EncodeAddress()
	events.Notify(events.EVENT_NEW_ADDRESS, addressStr)
	return addressStr
}
