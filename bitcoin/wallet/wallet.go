package wallet

func Start() {
	initStorage()
	startWatchingWalletUpdates()
	CheckForWalletUpdates()
}
