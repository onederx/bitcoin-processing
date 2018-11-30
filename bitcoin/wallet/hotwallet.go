package wallet

import (
	"errors"
	"github.com/onederx/bitcoin-processing/settings"
	"log"
)

func (w *Wallet) generateHotWalletAddress() (string, error) {
	newHotWalletAddress, err := w.generateNewAddress()
	if err != nil {
		return "", errors.New(
			"Error generating hot wallet address " + err.Error(),
		)
	}
	err = w.storage.SetHotWalletAddress(newHotWalletAddress)
	if err != nil {
		return "", errors.New(
			"Error storing generated hot wallet address " + err.Error(),
		)
	}
	return newHotWalletAddress, nil
}

func (w *Wallet) getOrCreateHotWallet() string {
	addressFromSettings := settings.GetString("wallet.hot_wallet_address")
	if addressFromSettings != "" {
		log.Printf(
			"Using hot wallet address from config: %s",
			addressFromSettings,
		)
		return addressFromSettings
	}
	addressFromStorage := w.storage.GetHotWalletAddress()
	if addressFromStorage != "" {
		log.Printf(
			"Loaded hot wallet address from storage: %s",
			addressFromStorage,
		)
		return addressFromStorage
	}
	newHotWalletAddress, err := w.generateHotWalletAddress()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Generated a new hot wallet address %s", newHotWalletAddress)
	return newHotWalletAddress
}

func (w *Wallet) checkHotWalletAddress() {
	addressInfo, err := w.nodeAPI.GetAddressInfo(w.hotWalletAddress)
	if err != nil {
		log.Fatalf(
			"Error: failed to check hot wallet address %s: %s",
			w.hotWalletAddress,
			err,
		)
	}
	if !addressInfo.IsMine {
		log.Fatalf(
			"Error checking hot wallet address %s: address does not belong "+
				"to wallet",
			w.hotWalletAddress,
		)
	}
	log.Printf("Checking hot wallet address: OK")
}

func (w *Wallet) GetHotWalletAddress() string {
	return w.hotWalletAddress
}

func (w *Wallet) initHotWallet() {
	w.hotWalletAddress = w.getOrCreateHotWallet()
	w.checkHotWalletAddress()
}
