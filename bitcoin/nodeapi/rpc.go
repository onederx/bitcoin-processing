package nodeapi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/settings"
)

type NodeAPI struct {
	btcrpc  *rpcclient.Client
	address string
	nodeURL string
	user    string
	pass    string
	useTLS  bool

	// Sending money consists of several RPC calls (SetTxFee is done before
	// SendToAddress for per KB fee and sending with fixed fee is done using
	// many RPC calls working with raw transactions)
	// prevent commands from racing (trying to double-spend UTXOs or
	// re-setting fee etc) by holding a lock while creating outgoing payment
	moneySendLock sync.Mutex
}

type JsonRPCError struct {
	Code    int
	Message string
}

type AddressInfo struct {
	Address      string
	ScriptPubKey string
	IsMine       bool `json:"ismine"`
	IsWatchonly  bool `json:"iswatchonly"`
	IsScript     bool `json:"isscript"`
	IsWitness    bool `json:"iswitness"`
	Script       string
	Hex          string
	Pubkey       string
	Embedded     struct {
		IsScript       bool   `json:"isscript"`
		IsWitness      bool   `json:"iswitness"`
		WitnessVersion bool   `json:"witness_version"`
		WitnessProgram string `json:"witness_program"`
		Pubkey         string
		Address        string
		ScriptPubKey   string
	}
	Label         string
	Timestamp     uint64
	HdKeyPath     string `json:"hdkeypath"`
	HdSeedId      string `json:"hdseedid"`
	HdMasterKeyId string `json:"hdmasterkeyid"`
	Labels        []struct {
		Name    string
		Purpose string
	}
}

type jsonRPCRequest struct {
	JSONRPCVersion string        `json:"jsonrpc"`
	Method         string        `json:"method"`
	Params         []interface{} `json:"params"`
}

type jsonRPCStringResponse struct {
	Result string
	Error  *JsonRPCError
}

type fundRawTransactionOptions struct {
	ChangePosition         int     `json:"changePosition"`
	FeeRate                float64 `json:"feeRate"`
	SubtractFeeFromOutputs []int   `json:"subtractFeeFromOutputs"`
}

type fundRawTransactionResult struct {
	Changepos int
	Fee       float64
	Hex       string
}

func (err JsonRPCError) Error() string {
	return fmt.Sprintf(
		"Bitcoin node returned error code %d message %s",
		err.Code,
		err.Message,
	)
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

func (n *NodeAPI) decodeRawTransaction(rawTxHex string) (*btcjson.TxRawResult, error) {
	rawTxBytes, err := hex.DecodeString(rawTxHex)
	if err != nil {
		return nil, err
	}
	return n.btcrpc.DecodeRawTransaction(rawTxBytes)
}

func (n *NodeAPI) GetRawTransaction(hash string) (*btcjson.TxRawResult, error) {
	transaction, err := n.GetTransaction(hash)
	if err != nil {
		return nil, err
	}
	return n.decodeRawTransaction(transaction.Hex)
}

func (n *NodeAPI) sendRequestToNode(method string, params []interface{}) ([]byte, error) {
	rpcRequest := jsonRPCRequest{
		JSONRPCVersion: "1.0",
		Method:         method,
		Params:         params,
	}
	rpcRequestJSON, err := json.Marshal(rpcRequest)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequest(
		"POST",
		n.nodeURL,
		bytes.NewReader(rpcRequestJSON),
	)
	if err != nil {
		return nil, err
	}
	httpRequest.Header["Content-Type"] = []string{"application/json"}
	httpRequest.SetBasicAuth(n.user, n.pass)
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return ioutil.ReadAll(response.Body)
}

func (n *NodeAPI) sendToAddress(address string, amount uint64, recipientPaysFee bool) (hash string, err error) {
	// there is SendToAddress in btcd/rpcclient, but it does not have
	// "Subtract Fee From Amount" argument
	var response jsonRPCStringResponse
	responseJSON, err := n.sendRequestToNode(
		"sendtoaddress",
		[]interface{}{
			address,
			btcutil.Amount(amount).ToBTC(),
			"",
			"",
			recipientPaysFee,
		},
	)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(responseJSON, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) SendWithPerKBFee(address string, amount uint64, fee uint64, recipientPaysFee bool) (hash string, err error) {
	n.moneySendLock.Lock()
	defer n.moneySendLock.Unlock()

	err = n.btcrpc.SetTxFee(btcutil.Amount(fee))

	if err != nil {
		return "", err
	}
	return n.sendToAddress(address, amount, recipientPaysFee)
}

func (n *NodeAPI) createRawTransaction(inputs []btcjson.TransactionInput, outputs map[string]float64) (string, error) {
	// there is CreateRawTransaction in btcd/rpcclient, but it does not work
	// with empty list of inputs. Wireshark shows that the request itself is
	// successful and node correctly returns JSON with created transaction,
	// but rpcclient later fails on parsing the result and returns error
	var response jsonRPCStringResponse
	createRawTxJSONResp, err := n.sendRequestToNode(
		"createrawtransaction",
		[]interface{}{inputs, outputs},
	)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(createRawTxJSONResp, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) getRawChangeAddress() (string, error) {
	// there is no GetRawChangeAddress in btcd/rpcclient, but it doesn't work:
	// it accepts one string argument "account" while real
	// createrawchangeaddress RPC call does not accept it - which results in
	// error
	var response jsonRPCStringResponse
	responseJSON, err := n.sendRequestToNode("getrawchangeaddress", nil)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(responseJSON, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) fundRawTransaction(rawTx string, options *fundRawTransactionOptions) (*fundRawTransactionResult, error) {
	// there is no FundRawTransaction in btcd/rpcclient
	var response struct {
		Result *fundRawTransactionResult
		Error  *JsonRPCError
	}
	fundRawTxJSONResp, err := n.sendRequestToNode(
		"fundrawtransaction",
		[]interface{}{rawTx, options},
	)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(fundRawTxJSONResp, &response)
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) transformTxToSetFixedFee(rawTxFunded *fundRawTransactionResult, address string, fixedFee uint64) (*btcjson.TxRawResult, error) {
	const errorPrefix = "Failed to transform tx to set fixed fee: "
	var recipientPos, expectedOutputNumber int
	decodedRawTx, err := n.decodeRawTransaction(rawTxFunded.Hex)

	if err != nil {
		return nil, errors.New(fmt.Sprintf(
			"Failed to decode tx %s: %s",
			string(rawTxFunded.Hex),
			err,
		))
	}

	changePos := rawTxFunded.Changepos
	autoFee, err := btcutil.NewAmount(rawTxFunded.Fee)

	if err != nil {
		return nil, err
	}

	if changePos != -1 { // change output is present
		recipientPos = 1
		expectedOutputNumber = 2
		if changePos != 0 {
			log.Printf(
				"Warning: unexpected change position for tx %#v: expected to "+
					"be 0 as requested or -1 (if no change is present).",
				rawTxFunded,
			)
			recipientPos = 0
		}
	} else { // no change
		recipientPos = 0
		expectedOutputNumber = 1
	}
	if len(decodedRawTx.Vout) != expectedOutputNumber {
		return nil, errors.New(fmt.Sprintf(
			errorPrefix+"expected exacly %d outputs for tx %#v",
			rawTxFunded,
		))
	}
	recipientOut := &decodedRawTx.Vout[recipientPos]

	if len(recipientOut.ScriptPubKey.Addresses) != 1 {
		return nil, errors.New(fmt.Sprintf(
			errorPrefix+"expected that recipient output will contain one "+
				"destination address, but it contains %d. Tx %#v",
			len(recipientOut.ScriptPubKey.Addresses),
			decodedRawTx,
		))
	}
	if recipientOut.ScriptPubKey.Addresses[0] != address {
		return nil, errors.New(fmt.Sprintf(
			errorPrefix+"address %s in recipient output does not match "+
				"destination address of payment %s. Tx %#v",
			recipientOut.ScriptPubKey.Addresses[0],
			address,
			decodedRawTx,
		))
	}
	recipientOutAmount, err := btcutil.NewAmount(recipientOut.Value)
	if err != nil {
		return nil, err
	}
	recipientOutAmount = recipientOutAmount - btcutil.Amount(fixedFee) + autoFee
	recipientOut.Value = recipientOutAmount.ToBTC()
	return decodedRawTx, nil
}

func (n *NodeAPI) encodeTransformedTransaction(tx *btcjson.TxRawResult) (string, error) {
	finalInputs := make([]btcjson.TransactionInput, len(tx.Vin))
	finalOutputs := make(map[string]float64)

	for i := range tx.Vin {
		finalInputs[i].Txid = tx.Vin[i].Txid
		finalInputs[i].Vout = tx.Vin[i].Vout
	}
	for i := range tx.Vout {
		if len(tx.Vout[i].ScriptPubKey.Addresses) != 1 {
			return "", errors.New(fmt.Sprintf(
				"Expected that tx outputs will have 1 destination address, "+
					"but %#v has %d. Tx %#v",
				tx.Vout[i],
				len(tx.Vout[i].ScriptPubKey.Addresses),
				tx,
			))
		}
		destinationAddress := tx.Vout[i].ScriptPubKey.Addresses[0]
		finalOutputs[destinationAddress] = tx.Vout[i].Value
	}
	return n.createRawTransaction(finalInputs, finalOutputs)
}

func (n *NodeAPI) signRawTransactionWithWallet(rawTx string) (string, error) {
	var response struct {
		Result *struct {
			Complete bool
			Hex      string
		}
		Error *JsonRPCError
	}
	signRawTxJSONResp, err := n.sendRequestToNode(
		"signrawtransactionwithwallet",
		[]interface{}{rawTx},
	)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(signRawTxJSONResp, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result.Hex, nil
}

func (n *NodeAPI) sendRawTransaction(rawTx string) (string, error) {
	var response jsonRPCStringResponse
	sendRawTxJSONResp, err := n.sendRequestToNode(
		"sendrawtransaction",
		[]interface{}{rawTx},
	)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(sendRawTxJSONResp, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) SendWithFixedFee(address string, amountSatoshi uint64, fee uint64, recipientPaysFee bool) (hash string, err error) {
	var amount btcutil.Amount
	n.moneySendLock.Lock()
	defer n.moneySendLock.Unlock()
	if recipientPaysFee {
		if amountSatoshi < fee {
			return "", errors.New(fmt.Sprintf(
				"Error: Recipient (%s) should pay fee %d satoshi, but amount sent"+
					" is less: %d satoshi",
				address,
				fee,
				amountSatoshi,
			))
		}
		amount = btcutil.Amount(amountSatoshi)
	} else {
		amount = btcutil.Amount(amountSatoshi + fee)
	}
	rawTx, err := n.createRawTransaction(
		[]btcjson.TransactionInput{}, // empty array: no inputs
		map[string]float64{address: amount.ToBTC()},
	)

	if err != nil {
		return "", err
	}

	rawTxFunded, err := n.fundRawTransaction(rawTx, &fundRawTransactionOptions{
		FeeRate:                bitcoin.MinimalFeeRateBTC,
		SubtractFeeFromOutputs: []int{0},
		ChangePosition:         0,
	})

	if err != nil {
		return "", err
	}

	transformedTx, err := n.transformTxToSetFixedFee(rawTxFunded, address, fee)

	if err != nil {
		return "", err
	}

	transformedTxEncoded, err := n.encodeTransformedTransaction(transformedTx)

	if err != nil {
		return "", err
	}

	signedTx, err := n.signRawTransactionWithWallet(transformedTxEncoded)

	if err != nil {
		return "", err
	}

	return n.sendRawTransaction(signedTx)
}

func (n *NodeAPI) GetAddressInfo(address string) (*AddressInfo, error) {
	// there is no GetAddressInfo in btcd/rpcclient

	var response struct {
		Result *AddressInfo
		Error  *JsonRPCError
	}
	getAddressInfoJSONResp, err := n.sendRequestToNode(
		"getaddressinfo",
		[]interface{}{address},
	)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(getAddressInfoJSONResp, &response)
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}

func NewNodeAPI() *NodeAPI {
	var nodeURLScheme string
	// prepare bitcoind RPC connection
	// Connect to remote bitcoin core RPC server using HTTP POST mode.
	host := settings.GetStringMandatory("bitcoin.node.address")
	user := settings.GetStringMandatory("bitcoin.node.user")
	pass := settings.GetStringMandatory("bitcoin.node.password")
	useTLS := settings.GetBool("bitcoin.node.tls")
	connCfg := &rpcclient.ConnConfig{
		Host:         host,
		User:         user,
		Pass:         pass,
		HTTPPostMode: true,    // Bitcoin core only supports HTTP POST mode
		DisableTLS:   !useTLS, // Bitcoin core can use TLS if it's behind a TLS proxy like nginx
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

	if useTLS {
		nodeURLScheme = "https"
	} else {
		nodeURLScheme = "http"
	}

	return &NodeAPI{
		address: host,
		nodeURL: fmt.Sprintf("%s://%s/", nodeURLScheme, host),
		user:    user,
		pass:    pass,
		useTLS:  useTLS,
		btcrpc:  btcrpc,
	}
}
