package wallet

import (
	"errors"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	settingstestutil "github.com/onederx/bitcoin-processing/settings/testutil"
)

const getAddressInfoError = "Failed to get address info"

type nodeAPIGetAddressInfoNotMineMock struct {
	nodeapi.NodeAPI
	t *testing.T
}

func (n *nodeAPIGetAddressInfoNotMineMock) GetAddressInfo(address string) (*nodeapi.AddressInfo, error) {
	if address != testAddress {
		n.t.Errorf("GetAddressInfo called with unexpected address %s", address)
	}
	return &nodeapi.AddressInfo{IsMine: false}, nil
}

type nodeAPIGetAddressInfoMineMock struct {
	nodeapi.NodeAPI
	t *testing.T
}

func (n *nodeAPIGetAddressInfoMineMock) GetAddressInfo(address string) (*nodeapi.AddressInfo, error) {
	if address != testAddress {
		n.t.Errorf("GetAddressInfo called with unexpected address %s", address)
	}
	return &nodeapi.AddressInfo{IsMine: true}, nil
}

type nodeAPIGetAddressInfoFailMock struct {
	nodeapi.NodeAPI
	t *testing.T
}

func (n *nodeAPIGetAddressInfoFailMock) GetAddressInfo(address string) (*nodeapi.AddressInfo, error) {
	if address != testAddress {
		n.t.Errorf("GetAddressInfo called with unexpected address %s", address)
	}
	return nil, errors.New(getAddressInfoError)
}

func TestColdWalletOK(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}
	s.Data["wallet.cold_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoNotMineMock{t: t},
		&eventBrokerMock{},
		NewStorage("memory", s),
	)
	w.initColdWallet()
	if got, want := w.coldWalletAddress, testAddress; got != want {
		t.Errorf("Unexpected cold wallet address: got %s instead of %s",
			got, want)
	}
}

func TestColdWalletOurOwnWallet(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}
	s.Data["wallet.cold_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoMineMock{t: t},
		&eventBrokerMock{},
		NewStorage("memory", s),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("initColdWallet() did not panic on our own address")
		}
	}()
	w.initColdWallet()
}

func TestColdWalletCheckFail(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}
	s.Data["wallet.cold_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoFailMock{t: t},
		&eventBrokerMock{},
		NewStorage("memory", s),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("initColdWallet() did not panic on address check error")
		}
	}()
	w.initColdWallet()
}

func TestColdWalletNotConfigured(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}
	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoNotMineMock{t: t},
		&eventBrokerMock{},
		NewStorage("memory", s),
	)
	w.initColdWallet()
	if got, want := w.coldWalletAddress, ""; got != want {
		t.Errorf("Cold wallet address is unexpectedly non-empty: got %s", got)
	}
}
