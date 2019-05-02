package testenv

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
	"github.com/onederx/bitcoin-processing/integrationtests/util"
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

func (e *TestEnvironment) startRegtest(ctx context.Context) error {
	log.Printf("Starting Regtest nodes")

	containerConfig := &container.Config{Image: bitcoinNodeImageName}
	e.Regtest = make(map[string]*bitcoinNodeContainerInfo)

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
				ID:   resp.ID,
			},
		}
		err = e.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
		if err != nil {
			e.stopRegtest(ctx) // in case other nodes were started
			return err
		}
		e.Regtest[node] = nodeContainerInfo
		nodeContainerInfo.ip = e.getContainerIP(ctx, resp.ID)

		err = e.writeContainerLogs(ctx, &nodeContainerInfo.containerInfo, node+".log")

		if err != nil {
			e.stopRegtest(ctx) // in case other nodes were started
			return err
		}

		log.Printf("Regtest node %s started: id=%v", node, resp.ID)
	}
	e.regtestIsLoaded = make(chan error)
	go e.initRegtest()
	return nil
}

func (e *TestEnvironment) setProcessingAddressForNotifications(address string) {
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

	err := util.WaitForEvent(func() error {
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

	err := util.WaitForEvent(func() error {
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

func (e *TestEnvironment) initRegtest() {
	clientNode, err := connectToNodeWithBackoff(e.Regtest["node-client"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.Regtest["node-client"].NodeAPI = clientNode
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
	minerNode, err := connectToNodeWithBackoff(e.Regtest["node-miner"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.Regtest["node-miner"].NodeAPI = minerNode
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
	ourNode, err := connectToNodeWithBackoff(e.Regtest["node-our"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.Regtest["node-our"].NodeAPI = ourNode
	e.regtestIsLoaded <- nil
}

func (e *TestEnvironment) WaitForContainerRemoval(ctx context.Context, containerID string) error {
	return util.WaitForEvent(func() error {
		_, err := e.cli.ContainerInspect(ctx, containerID)

		if err != nil {
			return nil
		}
		return fmt.Errorf("Container %s is still running", containerID)
	})
}

func (e *TestEnvironment) stopRegtest(ctx context.Context) error {
	log.Printf("trying to stop Regtest containers")
	if e.Regtest == nil {
		log.Printf("seems that Regtest is not running")
		return nil
	}

	for _, container := range e.Regtest {
		if err := e.cli.ContainerStop(ctx, container.ID, nil); err != nil {
			return err
		}
		log.Printf("Regtest container stopped: id=%v", container.ID)
	}
	e.Regtest = nil
	if e.notifyScriptFile != nil {
		e.notifyScriptFile.Close()
		e.notifyScriptFile = nil
	}
	return nil
}

func (e *TestEnvironment) waitForRegtest() {
	log.Printf("waiting for Regtest to start and load")
	err := <-e.regtestIsLoaded
	if err != nil {
		panic(err)
	}
	log.Printf("Regtest ready")
}

func GenerateBlocks(nodeAPI nodeapi.NodeAPI, amount int) ([]string, error) {
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

func (e *TestEnvironment) MineTx(txHash string) (string, error) {
	miner := e.Regtest["node-miner"].NodeAPI
	log.Printf("Mine tx: waiting for tx %s to get into miner mempool", txHash)
	err := util.WaitForEvent(func() error {
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
	generatedBlocks, err := GenerateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *TestEnvironment) MineAnyTx() (string, error) {
	miner := e.Regtest["node-miner"].NodeAPI
	log.Print("Mine tx: waiting for any tx to get into miner mempool")
	err := util.WaitForEvent(func() error {
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
	generatedBlocks, err := GenerateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *TestEnvironment) MineMultipleTxns(txHashes []string) (string, error) {
	miner := e.Regtest["node-miner"].NodeAPI
	log.Printf("Mine tx: waiting for txns %v to get into miner mempool", txHashes)
	err := util.WaitForEvent(func() error {
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
	generatedBlocks, err := GenerateBlocks(miner, 1)
	if err != nil {
		return "", err
	}
	return generatedBlocks[0], nil
}

func (e *TestEnvironment) GetNodeBalance(node nodeapi.NodeAPI) (*api.BalanceInfo, error) {
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

func (e *TestEnvironment) GetClientBalance() (*api.BalanceInfo, error) {
	return e.GetNodeBalance(e.Regtest["node-client"].NodeAPI)
}
