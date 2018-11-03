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
	maxConfirmations int64
}

func NewWallet(nodeAPI *nodeapi.NodeAPI, eventBroker *events.EventBroker) *Wallet {
	storageType := settings.GetStringMandatory("storage.type")
	maxConfirmations := int64(settings.GetInt("transaction.max-confirmations"))
	return &Wallet{
		nodeAPI:          nodeAPI,
		eventBroker:      eventBroker,
		storage:          newStorage(storageType),
		maxConfirmations: maxConfirmations,
	}
}

func (w *Wallet) Run() {
	w.checkForWalletUpdates()
	w.startWatchingWalletUpdates()
}
