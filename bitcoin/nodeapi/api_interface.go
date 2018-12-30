package nodeapi

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcutil"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

// NodeAPI is responsible for communication with Bitcoin node
type NodeAPI interface {
	CreateNewAddress() (btcutil.Address, error)
	ListTransactionsSinceBlock(blockHash string) (*btcjson.ListSinceBlockResult, error)
	GetTransaction(hash string) (*btcjson.GetTransactionResult, error)
	GetRawTransaction(hash string) (*btcjson.TxRawResult, error)
	SendWithPerKBFee(address string, amount, fee bitcoin.BTCAmount, recipientPaysFee bool) (hash string, err error)
	SendWithFixedFee(address string, amount, fee bitcoin.BTCAmount, recipientPaysFee bool) (hash string, err error)
	GetAddressInfo(address string) (*AddressInfo, error)
	GetConfirmedAndUnconfirmedBalance() (uint64, uint64, error)

	SendRequestToNode(method string, params []interface{}) ([]byte, error)
}
