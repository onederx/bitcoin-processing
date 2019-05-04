package wallet

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/events"
)

// Account describes user account. It consists of bitcoin address and metainfo
// supplied when account was created
type Account struct {
	Address  string                 `json:"address"`
	Metainfo map[string]interface{} `json:"metainfo"`
}

func init() {
	events.RegisterNotificationUnmarshaler(events.NewAddressEvent, func(b []byte) (interface{}, error) {
		var account Account

		err := json.Unmarshal(b, &account)
		return &account, err
	})
}

func (w *Wallet) generateNewAddress() (string, error) {
	return w.nodeAPI.CreateNewAddress()
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
	w.eventBroker.Notify(events.NewAddressEvent, account)
	return account, nil
}
