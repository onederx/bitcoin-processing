package wallet

import (
	"errors"
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

const testAddress = "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX"
const createAddressFailedErrorMsg = "Failed to generate address"

var testMetainfo = map[string]interface{}{
	"testing": "testtest",
	"index":   123,
	"data":    map[string]string{"used": "tester"},
}

type nodeAPICreateNewAddressMock struct {
	nodeapi.NodeAPI
}

func (n *nodeAPICreateNewAddressMock) CreateNewAddress() (btcutil.Address, error) {
	return btcutil.DecodeAddress(testAddress, &chaincfg.MainNetParams)
}

type nodeAPICreateNewAddressErrorMock struct {
	nodeapi.NodeAPI
}

func (n *nodeAPICreateNewAddressErrorMock) CreateNewAddress() (btcutil.Address, error) {
	return nil, errors.New(createAddressFailedErrorMsg)
}

type settingsMock struct {
	settings.Settings

	data map[string]interface{}
}

type eventBrokerMock struct {
	events.EventBroker
}

func (s *settingsMock) GetStringMandatory(key string) string {
	val, ok := s.data[key]

	if !ok {
		return ""
	}
	st, ok := val.(string)

	if !ok {
		return ""
	}
	return st
}

func (s *settingsMock) GetInt(key string) int {
	val, ok := s.data[key]

	if !ok {
		return 0
	}
	i, ok := val.(int)

	if !ok {
		return 0
	}
	return i
}

func (s *settingsMock) GetBTCAmount(key string) bitcoin.BTCAmount {
	val, ok := s.data[key]

	if !ok {
		return bitcoin.BTCAmount(0)
	}

	a, ok := val.(bitcoin.BTCAmount)

	if !ok {
		return bitcoin.BTCAmount(0)
	}
	return a
}

func TestCreateAccountSuccessful(t *testing.T) {
	s := &settingsMock{data: make(map[string]interface{})}
	s.data["storage.type"] = "memory"
	w := NewWallet(s, &nodeAPICreateNewAddressMock{}, &eventBrokerMock{})

	account, err := w.CreateAccount(testMetainfo)
	if err != nil {
		t.Errorf("CreateAccount returned error %s", err)
	}
	if got, want := account.Address, testAddress; got != want {
		t.Errorf("CreateAccount did not generated unexpected address %s", got)
	}
	if !reflect.DeepEqual(account.Metainfo, testMetainfo) {
		t.Errorf(
			"CreateAccount did not generated unexpected metainfo %v",
			account.Metainfo,
		)
	}
}

func TestCreateAccountAddressGenerationFailure(t *testing.T) {
	s := &settingsMock{data: make(map[string]interface{})}
	s.data["storage.type"] = "memory"
	w := NewWallet(s, &nodeAPICreateNewAddressErrorMock{}, &eventBrokerMock{})

	_, err := w.CreateAccount(testMetainfo)
	if err == nil {
		t.Errorf(
			"CreateAccount did not return error in case of address " +
				"generation failure",
		)
	}
	if got, want := err.Error(), createAddressFailedErrorMsg; got != want {
		t.Errorf("Unexpected error message %s", got)
	}
}
