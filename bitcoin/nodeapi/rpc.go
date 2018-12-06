package nodeapi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"

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
	txHashInChainhashFormat, err := chainhash.NewHashFromStr(hash)
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

	var response jsonRPCStringResponse
	err = json.Unmarshal(responseJSON, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) SendWithPerKBFee(address string, amount, fee bitcoin.BitcoinAmount,
	recipientPaysFee bool) (hash string, err error) {
	n.moneySendLock.Lock()
	defer n.moneySendLock.Unlock()

	err = n.btcrpc.SetTxFee(btcutil.Amount(fee))
	if err != nil {
		return "", err
	}

	return n.sendToAddress(address, uint64(amount), recipientPaysFee)
}

func (n *NodeAPI) createRawTransaction(inputs []btcjson.TransactionInput, outputs map[string]float64) (string, error) {
	// there is CreateRawTransaction in btcd/rpcclient, but it does not work
	// with empty list of inputs. Wireshark shows that the request itself is
	// successful and node correctly returns JSON with created transaction,
	// but rpcclient later fails on parsing the result and returns error
	createRawTxJSONResp, err := n.sendRequestToNode(
		"createrawtransaction",
		[]interface{}{inputs, outputs},
	)
	if err != nil {
		return "", err
	}

	var response jsonRPCStringResponse
	err = json.Unmarshal(createRawTxJSONResp, &response)
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
	fundRawTxJSONResp, err := n.sendRequestToNode(
		"fundrawtransaction",
		[]interface{}{rawTx, options},
	)
	if err != nil {
		return nil, err
	}

	var response struct {
		Result *fundRawTransactionResult
		Error  *JsonRPCError
	}
	err = json.Unmarshal(fundRawTxJSONResp, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) transformTxToSetFixedFee(rawTxFunded *fundRawTransactionResult, address string,
	fixedFee uint64) (*btcjson.TxRawResult, error) {
	const errorPrefix = "Failed to transform tx to set fixed fee: "
	decodedRawTx, err := n.decodeRawTransaction(rawTxFunded.Hex)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode tx %s: %s",
			string(rawTxFunded.Hex), err)
	}

	changePos := rawTxFunded.Changepos
	autoFee, err := btcutil.NewAmount(rawTxFunded.Fee)
	if err != nil {
		return nil, err
	}

	var recipientPos, expectedOutputNumber int
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
		return nil, fmt.Errorf(
			errorPrefix+"expected exacly %d outputs for tx %#v",
			rawTxFunded,
		)
	}

	recipientOut := &decodedRawTx.Vout[recipientPos]
	if len(recipientOut.ScriptPubKey.Addresses) != 1 {
		return nil, fmt.Errorf(errorPrefix+"expected that recipient output will contain one "+
			"destination address, but it contains %d. Tx %#v",
			len(recipientOut.ScriptPubKey.Addresses), decodedRawTx)
	}
	if recipientOut.ScriptPubKey.Addresses[0] != address {
		return nil, fmt.Errorf(errorPrefix+"address %s in recipient output does not match "+
			"destination address of payment %s. Tx %#v",
			recipientOut.ScriptPubKey.Addresses[0], address, decodedRawTx)
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
	for i := range tx.Vin {
		finalInputs[i].Txid = tx.Vin[i].Txid
		finalInputs[i].Vout = tx.Vin[i].Vout
	}

	finalOutputs := make(map[string]float64)
	for i := range tx.Vout {
		if len(tx.Vout[i].ScriptPubKey.Addresses) != 1 {
			return "", fmt.Errorf("Expected that tx outputs will have 1 destination address, "+
				"but %#v has %d. Tx %#v",
				tx.Vout[i], len(tx.Vout[i].ScriptPubKey.Addresses), tx)
		}
		destinationAddress := tx.Vout[i].ScriptPubKey.Addresses[0]
		finalOutputs[destinationAddress] = tx.Vout[i].Value
	}

	return n.createRawTransaction(finalInputs, finalOutputs)
}

func (n *NodeAPI) signRawTransactionWithWallet(rawTx string) (string, error) {
	signRawTxJSONResp, err := n.sendRequestToNode(
		"signrawtransactionwithwallet",
		[]interface{}{rawTx},
	)
	if err != nil {
		return "", err
	}

	var response struct {
		Result *struct {
			Complete bool
			Hex      string
		}
		Error *JsonRPCError
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
	sendRawTxJSONResp, err := n.sendRequestToNode(
		"sendrawtransaction",
		[]interface{}{rawTx},
	)
	if err != nil {
		return "", err
	}

	var response jsonRPCStringResponse
	err = json.Unmarshal(sendRawTxJSONResp, &response)
	if err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) SendWithFixedFee(address string, amount, fee bitcoin.BitcoinAmount,
	recipientPaysFee bool) (hash string, err error) {
	n.moneySendLock.Lock()
	defer n.moneySendLock.Unlock()

	if recipientPaysFee {
		if amount < fee {
			return "", fmt.Errorf(
				"Error: Recipient (%s) should pay fee %s satoshi, but amount sent"+
					" is less: %s",
				address, fee, amount)
		}
	} else {
		amount += fee
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

	transformedTx, err := n.transformTxToSetFixedFee(rawTxFunded, address, uint64(fee))
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

	getAddressInfoJSONResp, err := n.sendRequestToNode(
		"getaddressinfo",
		[]interface{}{address},
	)
	if err != nil {
		return nil, err
	}

	var response struct {
		Result *AddressInfo
		Error  *JsonRPCError
	}
	err = json.Unmarshal(getAddressInfoJSONResp, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}

func (n *NodeAPI) getBalance() (uint64, error) {
	balance, err := n.btcrpc.GetBalance("*")
	if err != nil {
		return 0, err
	}
	return uint64(balance), nil
}

func (n *NodeAPI) getUnconfirmedBalance() (uint64, error) {
	// there is GetUnconfirmedBalance in btcd/rpcclient, but it is broken:
	// it sends one positional argument while Bitcoin Core expects no args
	getUnconfirmedBalanceJSONResp, err := n.sendRequestToNode(
		"getunconfirmedbalance",
		nil,
	)
	if err != nil {
		return 0, err
	}

	var response struct {
		Result *float64
		Error  *JsonRPCError
	}
	err = json.Unmarshal(getUnconfirmedBalanceJSONResp, &response)
	if err != nil {
		return 0, err
	}
	if response.Error != nil {
		return 0, response.Error
	}
	amount, err := btcutil.NewAmount(*response.Result)
	if err != nil {
		return 0, err
	}
	return uint64(amount), nil
}

func (n *NodeAPI) GetConfirmedAndUnconfirmedBalance() (uint64, uint64, error) {
	n.moneySendLock.Lock()
	defer n.moneySendLock.Unlock()

	confirmedBalance, err := n.getBalance()
	if err != nil {
		return 0, 0, err
	}

	unconfirmedBalance, err := n.getUnconfirmedBalance()
	if err != nil {
		return 0, 0, err
	}

	return confirmedBalance, unconfirmedBalance, nil
}

func NewNodeAPI() *NodeAPI {
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
	btcrpc, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	blockCount, err := btcrpc.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Testing Bitcoin node connection: block count = %d", blockCount)

	var nodeURLScheme string
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
