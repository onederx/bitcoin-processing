package wallet

import (
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/settings"
)

type Wallet struct {
	nodeAPI          *nodeapi.NodeAPI
	eventBroker      *events.EventBroker
	storage          WalletStorage
	hotWalletAddress string
	minWithdraw      uint64
	minFeePerKb      uint64
	minFeeFixed      uint64
	maxConfirmations int64
}

func NewWallet(nodeAPI *nodeapi.NodeAPI, eventBroker *events.EventBroker) *Wallet {
	storageType := settings.GetStringMandatory("storage.type")
	maxConfirmations := int64(settings.GetInt("transaction.max-confirmations"))
	return &Wallet{
		nodeAPI:          nodeAPI,
		eventBroker:      eventBroker,
		storage:          newStorage(storageType),
		minWithdraw:      uint64(settings.GetInt64("wallet.min-withdraw")),
		minFeePerKb:      uint64(settings.GetInt64("wallet.min-fee.per-kb")),
		minFeeFixed:      uint64(settings.GetInt64("wallet.min-fee.fixed")),
		maxConfirmations: maxConfirmations,
	}
}

func (w *Wallet) Run() {
	w.initHotWallet()
	w.checkForWalletUpdates()
	w.updatePendingTxns()
	w.startWatchingWalletUpdates()
}
