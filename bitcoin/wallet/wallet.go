package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

const internalQueueSize = 10000

type Wallet struct {
	nodeAPI                              *nodeapi.NodeAPI
	eventBroker                          *events.EventBroker
	storage                              WalletStorage
	hotWalletAddress                     string
	coldWalletAddress                    string
	minWithdraw                          bitcoin.BitcoinAmount
	minFeePerKb                          bitcoin.BitcoinAmount
	minFeeFixed                          bitcoin.BitcoinAmount
	minWithdrawWithoutManualConfirmation bitcoin.BitcoinAmount
	maxConfirmations                     int64

	withdrawQueue chan internalWithdrawRequest
	cancelQueue   chan internalCancelRequest
}

func NewWallet(nodeAPI *nodeapi.NodeAPI, eventBroker *events.EventBroker) *Wallet {
	storageType := settings.GetStringMandatory("storage.type")
	maxConfirmations := int64(settings.GetInt("transaction.max_confirmations"))
	return &Wallet{
		nodeAPI:                              nodeAPI,
		eventBroker:                          eventBroker,
		storage:                              newStorage(storageType),
		minWithdraw:                          settings.GetBitcoinAmount("wallet.min_withdraw"),
		minFeePerKb:                          settings.GetBitcoinAmount("wallet.min_fee.per_kb"),
		minFeeFixed:                          settings.GetBitcoinAmount("wallet.min_fee.fixed"),
		minWithdrawWithoutManualConfirmation: settings.GetBitcoinAmount("wallet.min_withdraw_without_manual_confirmation"),
		maxConfirmations:                     maxConfirmations,
		withdrawQueue:                        make(chan internalWithdrawRequest, internalQueueSize),
		cancelQueue:                          make(chan internalCancelRequest, internalQueueSize),
	}
}

func (w *Wallet) Run() {
	w.initHotWallet()
	w.initColdWallet()
	w.checkForWalletUpdates()
	w.updatePendingTxns()
	w.startWatchingWalletUpdates()
}
