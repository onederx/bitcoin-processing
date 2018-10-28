package wallet

func Start() {
	initStorage()
	startWatchingWalletUpdates()
	checkForWalletUpdates()
}
