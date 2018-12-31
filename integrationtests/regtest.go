package integrationtests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/settings"
)

const (
	bitcoinNodeImageName = "kylemanna/bitcoind"

	nodeNamePrefix = "bitcoin-processing-integration-test-"

	regtestNodeUser     = "bitcoinrpc"
	regtestNodePassword = "TEST_BITCOIN_NODE_PASSWORD"
)

var bitcoinNodes = []string{
	"node-our",
	"node-client",
	"node-miner",
}

func (e *testEnvironment) startRegtest(ctx context.Context) error {
	log.Printf("Starting regtest nodes")

	containerConfig := &container.Config{Image: bitcoinNodeImageName}
	e.regtest = make(map[string]*containerInfo)

	for _, node := range bitcoinNodes {
		hostConfig := &container.HostConfig{
			NetworkMode: container.NetworkMode(e.network),
			AutoRemove:  true,
			Binds: []string{
				getFullSourcePath("integrationtests/testdata/regtest/"+node+"/bitcoin.conf") +
					":/bitcoin/.bitcoin/bitcoin.conf",
			},
		}
		containerName := nodeNamePrefix + node
		resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
		if err != nil {
			e.stopRegtest(ctx) // in case other nodes were started
			return err
		}
		nodeContainerInfo := &containerInfo{
			name: node,
			id:   resp.ID,
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
	go e.waitForRegtestLoadAndGenBitcoins()
	return nil
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
	var r interface{}
	var api nodeapi.NodeAPI
	retries := 120

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		api, r = connectToNode(host)
		if api != nil {
			return api, nil
		}
		retries--
		if retries > 0 {
			continue
		}
		break
	}
	return nil, fmt.Errorf("Failed to connect to node: %v", r)
}

func sendRequestToNodeWithBackoff(n nodeapi.NodeAPI, method string, params []interface{}) ([]byte, error) {
	var err error
	var responseJSON, result []byte

	var response struct {
		Result interface{}
		Error  *nodeapi.JSONRPCError
	}

	retries := 120
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		retries--
		if retries <= 0 {
			break
		}
		responseJSON, err = n.SendRequestToNode(method, params)

		if err != nil {
			continue
		}
		err = json.Unmarshal(responseJSON, &response)
		if err != nil {
			continue
		}
		if response.Error != nil {
			err = response.Error
			continue
		}
		result, err = json.MarshalIndent(response.Result, "", "    ")
		if err == nil {
			return result, nil
		}
	}
	return nil, err
}

func (e *testEnvironment) waitForRegtestLoadAndGenBitcoins() {
	clientNode, err := connectToNodeWithBackoff(e.regtest["node-client"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	nodeOutput, err := sendRequestToNodeWithBackoff(clientNode, "generate", []interface{}{3})
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	os.Stdout.Write(nodeOutput)
	minerNode, err := connectToNodeWithBackoff(e.regtest["node-miner"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	nodeOutput, err = sendRequestToNodeWithBackoff(minerNode, "generate", []interface{}{110})
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	os.Stdout.Write(nodeOutput)
	_, err = connectToNodeWithBackoff(e.regtest["node-our"].ip)
	if err != nil {
		e.regtestIsLoaded <- err
		return
	}
	e.regtestIsLoaded <- nil
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
