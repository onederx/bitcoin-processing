package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

const internalQueueSize = 10000

// Wallet is responsible for processing and storing payments. It stores
// transactions and accounts using its storage and uses bitcoin/nodeapi.NodeAPI
// to make requests directly to Bitcoin node. Wallet runs a persistent goroutine
// ("wallet updater goroutine") that polls bitcoin node for updates on watched
// txns and processes requests that make changes to wallet (making withdrawals,
// cancelling or confirming txns etc.)
type Wallet struct {
	nodeAPI                              *nodeapi.NodeAPI
	eventBroker                          *events.EventBroker
	storage                              Storage
	hotWalletAddress                     string
	coldWalletAddress                    string
	minWithdraw                          bitcoin.BTCAmount
	minFeePerKb                          bitcoin.BTCAmount
	minFeeFixed                          bitcoin.BTCAmount
	minWithdrawWithoutManualConfirmation bitcoin.BTCAmount
	maxConfirmations                     int64

	withdrawQueue chan internalWithdrawRequest
	cancelQueue   chan internalCancelRequest
	confirmQueue  chan internalConfirmRequest
}

// NewWallet creates new Wallet instance. It requires a NodeAPI instance to
// interact with Bitcoin node and EventBroker instance to work with events.
// It will also create new Storage, parameters for which will be read from
// settings
func NewWallet(nodeAPI *nodeapi.NodeAPI, eventBroker *events.EventBroker) *Wallet {
	storageType := settings.GetStringMandatory("storage.type")
	maxConfirmations := int64(settings.GetInt("transaction.max_confirmations"))
	minWithdrawWithoutManualConfirmation := settings.GetBTCAmount("wallet.min_withdraw_without_manual_confirmation")
	return &Wallet{
		nodeAPI:                              nodeAPI,
		eventBroker:                          eventBroker,
		storage:                              newStorage(storageType),
		minWithdraw:                          settings.GetBTCAmount("wallet.min_withdraw"),
		minFeePerKb:                          settings.GetBTCAmount("wallet.min_fee.per_kb"),
		minFeeFixed:                          settings.GetBTCAmount("wallet.min_fee.fixed"),
		minWithdrawWithoutManualConfirmation: minWithdrawWithoutManualConfirmation,
		maxConfirmations:                     maxConfirmations,
		withdrawQueue:                        make(chan internalWithdrawRequest, internalQueueSize),
		cancelQueue:                          make(chan internalCancelRequest, internalQueueSize),
		confirmQueue:                         make(chan internalConfirmRequest, internalQueueSize),
	}
}

// Run initializes and runs wallet updater goroutine. This function does not
// return, so should be run in a new goroutine
func (w *Wallet) Run() {
	w.initHotWallet()
	w.initColdWallet()
	w.checkForWalletUpdates()
	w.updatePendingTxns()
	w.startWatchingWalletUpdates()
}
