package nodeapi

import (
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"log"

	"github.com/onederx/bitcoin-processing/settings"
)

type NodeAPI struct {
	btcrpc *rpcclient.Client
}

func (n *NodeAPI) CreateNewAddress() (btcutil.Address, error) {
	return n.btcrpc.GetNewAddress("")
}

func (n *NodeAPI) ListTransactionsSinceBlock(blockHash string) (*btcjson.ListSinceBlockResult, error) {
	var blockHashInChainhashFormat *chainhash.Hash
	var err error

	if blockHash == "" {
		blockHashInChainhashFormat = nil
	} else {
		blockHashInChainhashFormat, err = chainhash.NewHashFromStr(blockHash)
		if err != nil {
			return nil, errors.New(
				"Error: ListTransactionsSinceBlock: failed to convert block " +
					"hash " + blockHash + " to chainhash format: " + err.Error(),
			)
		}
	}

	return n.btcrpc.ListSinceBlock(blockHashInChainhashFormat)
}

func (n *NodeAPI) GetTransaction(hash string) (*btcjson.GetTransactionResult, error) {
	var txHashInChainhashFormat *chainhash.Hash
	var err error

	txHashInChainhashFormat, err = chainhash.NewHashFromStr(hash)
	if err != nil {
		return nil, errors.New(
			"Error: GetTransaction: failed to convert tx hash " + hash +
				" to chainhash format: " + err.Error(),
		)
	}

	return n.btcrpc.GetTransaction(txHashInChainhashFormat)
}

func (n *NodeAPI) GetRawTransaction(hash string) (*btcjson.TxRawResult, error) {
	transaction, err := n.GetTransaction(hash)

	if err != nil {
		return nil, err
	}
	rawTxBytes, err := hex.DecodeString(transaction.Hex)
	if err != nil {
		return nil, err
	}
	return n.btcrpc.DecodeRawTransaction(rawTxBytes)
}

func NewNodeAPI() *NodeAPI {
	// prepare bitcoind RPC connection
	// Connect to remote bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         settings.GetStringMandatory("bitcoin.node.address"),
		User:         settings.GetStringMandatory("bitcoin.node.user"),
		Pass:         settings.GetStringMandatory("bitcoin.node.password"),
		HTTPPostMode: true,                                  // Bitcoin core only supports HTTP POST mode
		DisableTLS:   !settings.GetBool("bitcoin.node.tls"), // Bitcoin core can use TLS if it's behind a TLS proxy like nginx
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	var err error
	btcrpc, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	blockCount, err := btcrpc.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Testing Bitcoin node connection: block count = %d", blockCount)

	return &NodeAPI{btcrpc}
}
