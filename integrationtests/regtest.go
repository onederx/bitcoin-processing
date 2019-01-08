package integrationtests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/settings"
)

const (
	bitcoinNodeImageName = "kylemanna/bitcoind"

	nodeNamePrefix = "bitcoin-processing-integration-test-"

	regtestNodeUser     = "bitcoinrpc"
	regtestNodePassword = "TEST_BITCOIN_NODE_PASSWORD"

	nodeConfigTemplate = `printtoconsole=1
rpcuser=bitcoinrpc
rpcpassword=TEST_BITCOIN_NODE_PASSWORD
regtest=1
rpcallowip=0.0.0.0/0
{{.Additional}}

[regtest]
{{.Peers}}
`

	processingNotifyScriptTemplate = `#!/bin/bash -e
echo "notify about tx $1"
echo -e "GET /notify_wallet HTTP/1.1\r\nhost: {{.ProcessingAddress}}\r\nConnection: close\r\n\r\n" > /dev/tcp/{{.ProcessingAddress}}/8000
`
)

type nodeConfig struct {
	Peers      string
	Additional string
}

type rawMempoolResponse struct {
	Result []string
	Error  *nodeapi.JSONRPCError
}

var bitcoinNodes = []string{
	"node-our",
	"node-client",
	"node-miner",
}

func (e *testEnvironment) startRegtest(ctx context.Context) error {
	log.Printf("Starting regtest nodes")

	containerConfig := &container.Config{Image: bitcoinNodeImageName}
	e.regtest = make(map[string]*bitcoinNodeContainerInfo)

	for _, node := range bitcoinNodes {
		peers := make([]string, 0)
		for _, otherNode := range bitcoinNodes {
			if otherNode != node {
				peers = append(peers, "addnode="+nodeNamePrefix+otherNode)
			}
		}
		nodeConfigParams := nodeConfig{Peers: strings.Join(peers, "\n")}
		if node == "node-our" {
			nodeConfigParams.Additional = "walletnotify=/bin/bash /usr/share/notify-processing.sh %s"
		}
		configTempFile, err := ioutil.TempFile("", "")
		if err != nil {
			e.stopRegtest(ctx)
			return err
		}
		configTempFilePath := configTempFile.Name()

		defer os.Remove(configTempFilePath)
		defer configTempFile.Close()

		tmpl := template.Must(template.New("config").Parse(nodeConfigTemplate))
		tmpl.Execute(configTempFile, nodeConfigParams)

		bindMounts := []string{
			configTempFilePath + ":/bitcoin/.bitcoin/bitcoin.conf",
		}

		if node == "node-our" {
			notifyScriptTempFile, err := ioutil.TempFile("", "")
			if err != nil {
				e.stopRegtest(ctx)
				return err
			}
			notifyScriptTempFilePath := notifyScriptTempFile.Name()
			defer os.Remove(notifyScriptTempFilePath)
			e.notifyScriptFile = notifyScriptTempFile
			bindMounts = append(bindMounts, notifyScriptTempFilePath+":/usr/share/notify-processing.sh")
			bindMounts = append(bindMounts, getFullSourcePath("tools/curl")+":/usr/local/bin/notifyprocessing")
		}

		hostConfig := &container.HostConfig{
			NetworkMode: container.NetworkMode(e.network),
			AutoRemove:  true,
			Binds:       bindMounts,
		}
		containerName := nodeNamePrefix + node
		resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
		if err != nil {
			e.stopRegtest(ctx) // in case other nodes were started
			return err
		}
		nodeContainerInfo := &bitcoinNodeContainerInfo{
			containerInfo: containerInfo{
				name: node,
				id:   resp.ID,
			},
		}
		err = e.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
		if err != nil {
			e.stopRegtest(ctx) // in case other nodes were started
			return err
		}
		e.regtest[node] = nodeContainerInfo
		nodeContainerInfo.ip = e.getContainerIP(ctx, resp.ID)
		log.Printf("regtest node %s started: id=%v", node, resp.ID)
	}
	e.regtestIsLoaded = make(chan error)
	go e.initRegtest()
	return nil
}

func (e *testEnvironment) setProcessingAddressForNotifications(address string) {
	templateArgs := struct{ ProcessingAddress string }{ProcessingAddress: address}

	tmpl := template.Must(template.New("notifyscript").Parse(processingNotifyScriptTemplate))
	_, err := e.notifyScriptFile.Seek(0, 0)
	if err != nil {
		panic(fmt.Sprintf("Failed to seek on notify script file %v", err))
	}
	tmpl.Execute(e.notifyScriptFile, templateArgs)
}

type regtestNodeSettings struct {
	settings.Settings
	creds map[string]string
}

func (r *regtestNodeSettings) GetStringMandatory(key string) string {
	return r.creds[key]
}

func (r *regtestNodeSettings) GetBool(key string) bool {
	return false
}

func connectToNode(host string) (result nodeapi.NodeAPI, r interface{}) {
	nodeSettings := &regtestNodeSettings{
		creds: map[string]string{
			"bitcoin.node.address":  host + ":18443",
			"bitcoin.node.user":     regtestNodeUser,
			"bitcoin.node.password": regtestNodePassword,
		},
	}
	defer func() {
		if r = recover(); r != nil {
			return
		}
	}()
	return nodeapi.NewNodeAPI(nodeSettings), nil
}

func connectToNodeWithBackoff(host string) (nodeapi.NodeAPI, error) {
	var api nodeapi.NodeAPI

	err := waitForEvent(func() error {
		var r interface{}
		api, r = connectToNode(host)
		if api != nil {
			return nil
		}
		return fmt.Errorf("Failed to connect to node: %v", r)
	})
	return api, err
}

func sendRequestToNodeWithBackoff(n nodeapi.NodeAPI, method string, params []interface{}) ([]byte, error) {
	var result []byte

	var response struct {
		Result interface{}
		Error  *nodeapi.JSONRPCError
	}

	err := waitForEvent(func() error {
		responseJSON, err := n.SendRequestToNode(method, params)

		if err != nil {
			return err
		}
		err = json.Unmarshal(responseJSON, &response)
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		result, err = json.MarshalIndent(response.Result, "", "    ")
		return err
	})
	return result, err
}

func (e *testEnvironment) initRegtest() {
	clientNode, err := connectToNodeWithBackoff(e.regtest["node-client"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.regtest["node-client"].nodeAPI = clientNode
	nodeOutput, err := sendRequestToNodeWithBackoff(clientNode, "generate", []interface{}{3})
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	var blocks []string
	err = json.Unmarshal(nodeOutput, &blocks)
	if err != nil {
		panic(fmt.Sprintf("Client node API returned malformed JSON %s", nodeOutput))
	}
	log.Printf("Client node generated %d blocks", len(blocks))
	minerNode, err := connectToNodeWithBackoff(e.regtest["node-miner"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.regtest["node-miner"].nodeAPI = minerNode
	nodeOutput, err = sendRequestToNodeWithBackoff(minerNode, "generate", []interface{}{110})
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	err = json.Unmarshal(nodeOutput, &blocks)
	if err != nil {
		panic(fmt.Sprintf("Miner node API returned malformed JSON %s", nodeOutput))
	}
	log.Printf("Miner node generated %d blocks", len(blocks))
	ourNode, err := connectToNodeWithBackoff(e.regtest["node-our"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.regtest["node-our"].nodeAPI = ourNode
	e.regtestIsLoaded <- nil
}

func (e *testEnvironment) waitForContainerRemoval(ctx context.Context, containerID string) error {
	return waitForEvent(func() error {
		_, err := e.cli.ContainerInspect(ctx, containerID)

		if err != nil {
			return nil
		}
		return fmt.Errorf("Container %s is still running", containerID)
	})
}

func (e *testEnvironment) stopRegtest(ctx context.Context) error {
	log.Printf("trying to stop regtest containers")
	if e.regtest == nil {
		log.Printf("seems that regtest is not running")
		return nil
	}

	for _, container := range e.regtest {
		if err := e.cli.ContainerStop(ctx, container.id, nil); err != nil {
			return err
		}
		log.Printf("regtest container stopped: id=%v", container.id)
	}
	e.regtest = nil
	if e.notifyScriptFile != nil {
		e.notifyScriptFile.Close()
		e.notifyScriptFile = nil
	}
	return nil
}

func (e *testEnvironment) waitForRegtest() {
	log.Printf("waiting for regtest to start and load")
	err := <-e.regtestIsLoaded
	if err != nil {
		panic(err)
	}
	log.Printf("regtest ready")
}

func generateBlocks(nodeAPI nodeapi.NodeAPI, amount int) ([]string, error) {
	var response struct {
		Result []string
		Error  *nodeapi.JSONRPCError
	}
	responseJSON, err := nodeAPI.SendRequestToNode(
		"generate", []interface{}{amount},
	)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(responseJSON, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}

func (e *testEnvironment) mineTx(txHash string) (string, error) {
	miner := e.regtest["node-miner"].nodeAPI
	log.Printf("Mine tx: waiting for tx %s to get into miner mempool", txHash)
	err := waitForEvent(func() error {
		var response rawMempoolResponse
		responseJSON, err := miner.SendRequestToNode("getrawmempool", nil)
		if err != nil {
			return err
		}
		err = json.Unmarshal(responseJSON, &response)
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		for _, mempoolTxHash := range response.Result {
			if mempoolTxHash == txHash {
				return nil
			}
		}
		return fmt.Errorf("Tx %s not in miner mempool", txHash)
	})
	if err != nil {
		return "", err
	}
	log.Printf("Mine tx: tx %s is in miner mempool generating new block", txHash)
	generatedBlocks, err := generateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *testEnvironment) mineAnyTx() (string, error) {
	miner := e.regtest["node-miner"].nodeAPI
	log.Print("Mine tx: waiting for any tx to get into miner mempool")
	err := waitForEvent(func() error {
		var response rawMempoolResponse
		responseJSON, err := miner.SendRequestToNode("getrawmempool", nil)
		if err != nil {
			return err
		}
		err = json.Unmarshal(responseJSON, &response)
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		if len(response.Result) > 0 {
			return nil
		}
		return errors.New("Miner mempool is empty")
	})
	if err != nil {
		return "", err
	}
	generatedBlocks, err := generateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *testEnvironment) mineMultipleTxns(txHashes []string) (string, error) {
	miner := e.regtest["node-miner"].nodeAPI
	log.Printf("Mine tx: waiting for txns %v to get into miner mempool", txHashes)
	err := waitForEvent(func() error {
		var response rawMempoolResponse
		responseJSON, err := miner.SendRequestToNode("getrawmempool", nil)
		if err != nil {
			return err
		}
		err = json.Unmarshal(responseJSON, &response)
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		for _, hash := range txHashes {
			found := false
			for _, mempoolTxHash := range response.Result {
				if mempoolTxHash == hash {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Tx %s not in miner mempool", hash)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	generatedBlocks, err := generateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *testEnvironment) getNodeBalance(node nodeapi.NodeAPI) (*api.BalanceInfo, error) {
	balanceConfU64, balanceUnconfU64, err := node.GetConfirmedAndUnconfirmedBalance()

	if err != nil {
		return nil, err
	}

	result := &api.BalanceInfo{
		Balance:           bitcoin.BTCAmount(balanceConfU64),
		BalanceWithUnconf: bitcoin.BTCAmount(balanceConfU64 + balanceUnconfU64),
	}
	return result, nil
}

func (e *testEnvironment) getClientBalance() (*api.BalanceInfo, error) {
	return e.getNodeBalance(e.regtest["node-client"].nodeAPI)
}
