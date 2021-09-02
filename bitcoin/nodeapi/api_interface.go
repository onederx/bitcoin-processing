package nodeapi

import (
	"github.com/btcsuite/btcd/btcjson"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

// NodeAPI is responsible for communication with Bitcoin node
type NodeAPI interface {
	CreateNewAddress() (string, error)
	CreateWallet(name string) error
	ListTransactionsSinceBlock(blockHash string) (*btcjson.ListSinceBlockResult, error)
	GetTransaction(hash string) (*btcjson.GetTransactionResult, error)
	GetRawTransaction(hash string) (*btcjson.TxRawResult, error)
	SendWithPerKBFee(address string, amount, fee bitcoin.BTCAmount, recipientPaysFee bool) (hash string, err error)
	SendWithFixedFee(address string, amount, fee bitcoin.BTCAmount, recipientPaysFee bool) (hash string, err error)
	SendToMultipleAddresses(addresses map[string]bitcoin.BTCAmount) (hash string, err error)
	GetAddressInfo(address string) (*AddressInfo, error)
	GetConfirmedAndUnconfirmedBalance() (uint64, uint64, error)

	SendRequestToNode(method string, params []interface{}) ([]byte, error)
	SendRequestToNodeWithNamedParams(method string, params map[string]interface{}) ([]byte, error)
}
