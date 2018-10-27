package nodeapi

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"log"

	"github.com/onederx/bitcoin-processing/config"
)

var (
	btcrpc *rpcclient.Client
)

func CreateNewAddress() (btcutil.Address, error) {
	return btcrpc.GetNewAddress("")
}

func InitBTCRPC() {
	// prepare bitcoind RPC connection
	// Connect to remote bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         config.GetStringMandatory("bitcoin.node.address"),
		User:         config.GetStringMandatory("bitcoin.node.user"),
		Pass:         config.GetStringMandatory("bitcoin.node.password"),
		HTTPPostMode: true,                                // Bitcoin core only supports HTTP POST mode
		DisableTLS:   !config.GetBool("bitcoin.node.tls"), // Bitcoin core can use TLS if it's behind a TLS proxy like nginx
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	var err error
	btcrpc, err = rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	blockCount, err := btcrpc.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Testing Bitcoin node connection: block count = %d", blockCount)
}
