package wallet

import (
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	settingstestutil "github.com/onederx/bitcoin-processing/settings/testutil"
)

const testAddress2 = "3QJmV3qfvL9SuYo34YihAf3sRCW3qSinyC"
const testAddressRegtest = "2NAvKkyAJK7EQChnSCNyWo4ALX5LQt1A4tL"

type nodeAPICreateNewAddressAndGetAddressInfoMineMock struct {
	nodeAPICreateNewAddressMock
	t *testing.T
}

func (n *nodeAPICreateNewAddressAndGetAddressInfoMineMock) GetAddressInfo(address string) (*nodeapi.AddressInfo, error) {
	if address != n.address {
		n.t.Errorf("GetAddressInfo called with unexpected address %s", address)
	}
	return &nodeapi.AddressInfo{IsMine: true}, nil
}

func TestHotWalletGenerate(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}

	w := NewWallet(
		s,
		&nodeAPICreateNewAddressAndGetAddressInfoMineMock{
			nodeAPICreateNewAddressMock: nodeAPICreateNewAddressMock{
				address: testAddress2,
			},
			t: t,
		},
		&eventBrokerMock{},
		NewStorage(nil),
	)
	w.initHotWallet()
	if got, want := w.GetHotWalletAddress(), testAddress2; got != want {
		t.Errorf("Unexpected generated hot wallet address: got %s instead of %s",
			got, want)
	}
}

func TestHotWalletGenerateRegtest(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}

	w := NewWallet(
		s,
		&nodeAPICreateNewAddressAndGetAddressInfoMineMock{
			nodeAPICreateNewAddressMock: nodeAPICreateNewAddressMock{
				address: testAddressRegtest,
			},
			t: t,
		},
		&eventBrokerMock{},
		NewStorage(nil),
	)
	w.initHotWallet()
	if got, want := w.GetHotWalletAddress(), testAddressRegtest; got != want {
		t.Errorf("Unexpected generated hot wallet address: got %s instead of %s",
			got, want)
	}
}

func TestHotWalletConfigured(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}

	s.Data["wallet.hot_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoMineMock{t: t},
		&eventBrokerMock{},
		NewStorage(nil),
	)
	w.initHotWallet()
	if got, want := w.GetHotWalletAddress(), testAddress; got != want {
		t.Errorf("Unexpected generated hot wallet address: got %s instead of %s",
			got, want)
	}
}

func TestHotWalletNotMineFail(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}

	s.Data["wallet.hot_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoNotMineMock{t: t},
		&eventBrokerMock{},
		NewStorage(nil),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("initHotWallet() did not panic on alien address")
		}
	}()
	w.initHotWallet()
}

func TestHotWalletGetAddressInfoFail(t *testing.T) {
	s := &settingstestutil.SettingsMock{Data: make(map[string]interface{})}

	s.Data["wallet.hot_wallet_address"] = testAddress

	w := NewWallet(
		s,
		&nodeAPIGetAddressInfoFailMock{t: t},
		&eventBrokerMock{},
		NewStorage(nil),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("initHotWallet() did not panic on get address info error")
		}
	}()
	w.initHotWallet()
}
