package wallet

import (
	"github.com/onederx/bitcoin-processing/settings"
	"log"
)

func (w *Wallet) initColdWallet() {
	w.coldWalletAddress = settings.GetString("wallet.cold_wallet_address")

	if w.coldWalletAddress == "" {
		return
	}

	addressInfo, err := w.nodeAPI.GetAddressInfo(w.coldWalletAddress)
	if err != nil {
		log.Fatal(
			"Error: failed to check cold wallet address ",
			w.coldWalletAddress,
			err,
		)
	}
	if addressInfo.IsMine {
		log.Fatalf(
			"Error: configured cold wallet address %s belongs to current "+
				"wallet. This is likely an error because cold storage should be "+
				"in a separate wallet not controlled by this app.",
			w.coldWalletAddress,
		)
	}
}
