package wallet

import (
	"database/sql"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

const internalQueueSize = 10000

type walletData struct {
	settings                             settings.Settings
	nodeAPI                              nodeapi.NodeAPI
	database                             *sql.DB
	hotWalletAddress                     string
	coldWalletAddress                    string
	minWithdraw                          bitcoin.BTCAmount
	minFeePerKb                          bitcoin.BTCAmount
	minFeeFixed                          bitcoin.BTCAmount
	minWithdrawWithoutManualConfirmation bitcoin.BTCAmount
	maxConfirmations                     int64

	withdrawQueue           chan internalWithdrawRequest
	cancelQueue             chan internalCancelRequest
	confirmQueue            chan internalConfirmRequest
	holdQueue               chan internalHoldRequest
	externalTxNotifications chan struct{}
	pendingTxUpdateTrigger  chan struct{}
}

// Wallet is responsible for processing and storing payments. It stores
// transactions and accounts using its storage and uses bitcoin/nodeapi.NodeAPI
// to make requests directly to Bitcoin node. Wallet runs a persistent goroutine
// ("wallet updater goroutine") that polls bitcoin node for updates on watched
// txns and processes requests that make changes to wallet (making withdrawals,
// cancelling or confirming txns etc.)
type Wallet struct {
	*walletData
	eventBroker events.EventBroker
	storage     Storage
}

// NewWallet creates new Wallet instance. It requires a NodeAPI instance to
// interact with Bitcoin node and EventBroker instance to work with events.
// It will also create new Storage, parameters for which will be read from
// settings
func NewWallet(s settings.Settings, nodeAPI nodeapi.NodeAPI, eventBroker events.EventBroker, storage Storage) *Wallet {
	maxConfirmations := int64(s.GetInt("transaction.max_confirmations"))
	minWithdrawWithoutManualConfirmation := s.GetBTCAmount("wallet.min_withdraw_without_manual_confirmation")
	return &Wallet{
		storage:     storage,
		eventBroker: eventBroker,
		walletData: &walletData{
			settings:                             s,
			nodeAPI:                              nodeAPI,
			database:                             storage.GetDB(),
			minWithdraw:                          s.GetBTCAmount("wallet.min_withdraw"),
			minFeePerKb:                          s.GetBTCAmount("wallet.min_fee.per_kb"),
			minFeeFixed:                          s.GetBTCAmount("wallet.min_fee.fixed"),
			minWithdrawWithoutManualConfirmation: minWithdrawWithoutManualConfirmation,
			maxConfirmations:                     maxConfirmations,
			withdrawQueue:                        make(chan internalWithdrawRequest, internalQueueSize),
			cancelQueue:                          make(chan internalCancelRequest, internalQueueSize),
			confirmQueue:                         make(chan internalConfirmRequest, internalQueueSize),
			holdQueue:                            make(chan internalHoldRequest, internalQueueSize),
			externalTxNotifications:              make(chan struct{}, 3),
			pendingTxUpdateTrigger:               make(chan struct{}, 3),
		},
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
