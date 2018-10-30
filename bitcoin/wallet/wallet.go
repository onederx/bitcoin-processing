package wallet

func Start() {
	initStorage()
	checkForWalletUpdates()
	startWatchingWalletUpdates()
}
