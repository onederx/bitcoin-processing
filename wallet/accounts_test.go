package wallet

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	settingstestutil "github.com/onederx/bitcoin-processing/settings/testutil"
)

const testAddress = "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX"
const createAddressFailedErrorMsg = "Failed to generate address"
const storeAccountError = "Failed to store account"

var testMetainfo = map[string]interface{}{
	"testing": "testtest",
	"index":   123,
	"data":    map[string]string{"used": "tester"},
}

type nodeAPICreateNewAddressMock struct {
	nodeapi.NodeAPI
	address string
}

func (n *nodeAPICreateNewAddressMock) CreateNewAddress() (string, error) {
	address := n.address
	if address == "" {
		address = testAddress
	}
	return address, nil
}

type nodeAPICreateNewAddressErrorMock struct {
	nodeapi.NodeAPI
}

func (n *nodeAPICreateNewAddressErrorMock) CreateNewAddress() (string, error) {
	return "", errors.New(createAddressFailedErrorMsg)
}

type eventBrokerMock struct {
	events.EventBroker
}

func (b *eventBrokerMock) Notify(eventType events.EventType, data interface{}) error {
	return nil
}

func (b *eventBrokerMock) SendNotifications() {}

type accountStoreFailureMock struct {
	Storage
}

func (a *accountStoreFailureMock) GetDB() *sql.DB {
	return nil
}

func (s *accountStoreFailureMock) StoreAccount(account *Account) error {
	return errors.New(storeAccountError)
}

func TestCreateAccountSuccessful(t *testing.T) {
	s := &settingstestutil.SettingsMock{}

	w := NewWallet(
		s,
		&nodeAPICreateNewAddressMock{},
		&eventBrokerMock{},
		NewStorage(nil),
	)

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
	s := &settingstestutil.SettingsMock{}

	w := NewWallet(
		s,
		&nodeAPICreateNewAddressErrorMock{},
		&eventBrokerMock{},
		NewStorage(nil),
	)

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

func TestCreateAccountStorageFailure(t *testing.T) {
	s := &settingstestutil.SettingsMock{}

	w := NewWallet(
		s,
		&nodeAPICreateNewAddressMock{},
		&eventBrokerMock{},
		&accountStoreFailureMock{},
	)

	_, err := w.CreateAccount(testMetainfo)
	if err == nil {
		t.Errorf(
			"CreateAccount did not return error in case of storage failure")
	}
	if got, want := err.Error(), storeAccountError; got != want {
		t.Errorf("Unexpected error message %s", got)
	}
}
