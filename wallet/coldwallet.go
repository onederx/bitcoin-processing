package wallet

import (
	"fmt"
)

func (w *Wallet) initColdWallet() {
	w.coldWalletAddress = w.settings.GetString("wallet.cold_wallet_address")
	if w.coldWalletAddress == "" {
		return
	}

	addressInfo, err := w.nodeAPI.GetAddressInfo(w.coldWalletAddress)
	if err != nil {
		panic(fmt.Sprintf("Error: failed to check cold wallet address %s %s",
			w.coldWalletAddress, err))
	}
	if addressInfo.IsMine {
		panic(fmt.Sprintf("Error: configured cold wallet address %s belongs "+
			"to current wallet. This is likely an error because cold storage "+
			"should be in a separate wallet not controlled by this app.",
			w.coldWalletAddress))
	}
}
