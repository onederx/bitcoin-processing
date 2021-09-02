package testenv

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

const coldStorageContainerName = "node-cold-storage"

func (e *TestEnvironment) StartColdStorage(ctx context.Context) error {
	log.Printf("Starting cold storage node")

	containerConfig := &container.Config{Image: bitcoinNodeImageName}

	peers := make([]string, 3)
	for i, otherNode := range bitcoinNodes {
		peers[i] = "addnode=" + nodeNamePrefix + otherNode
	}
	nodeConfigParams := nodeConfig{
		Peers:      strings.Join(peers, "\n"),
		DepositFee: e.depositFee.Float64(),
	}

	configTempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	configTempFilePath := configTempFile.Name()

	defer os.Remove(configTempFilePath)
	defer configTempFile.Close()

	tmpl := template.Must(template.New("config").Parse(nodeConfigTemplate))
	tmpl.Execute(configTempFile, nodeConfigParams)

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(e.network),
		AutoRemove:  true,
		Binds:       []string{configTempFilePath + ":/bitcoin/.bitcoin/bitcoin.conf"},
	}
	containerName := nodeNamePrefix + coldStorageContainerName
	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
	if err != nil {
		return err
	}
	nodeContainerInfo := &bitcoinNodeContainerInfo{
		containerInfo: containerInfo{
			name: coldStorageContainerName,
			ID:   resp.ID,
		},
	}
	err = e.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	e.Regtest[coldStorageContainerName] = nodeContainerInfo
	nodeContainerInfo.IP = e.getContainerIP(ctx, resp.ID)
	log.Printf("cold storage node started: id=%v", resp.ID)
	return nil
}

func (e *TestEnvironment) StopColdStorage(ctx context.Context) error {
	log.Printf("trying to stop cold storage container")
	if e.Regtest == nil {
		log.Printf("seems that Regtest is not running")
		return nil
	}
	cont, ok := e.Regtest[coldStorageContainerName]

	if !ok {
		log.Printf("seems that cold storage container is not running")
		return nil
	}

	delete(e.Regtest, coldStorageContainerName)

	if err := e.cli.ContainerStop(ctx, cont.ID, nil); err != nil {
		return err
	}
	log.Printf("cold storage container stopped: id=%v", cont.ID)
	return nil
}

func (e *TestEnvironment) ColdStorageLoadAndGenerateAddress() string {
	csNode, err := connectToNodeWithBackoff(e.Regtest[coldStorageContainerName].IP)
	if err != nil {
		panic(err)
	}
	err = createWallet(csNode, "default")
	if err != nil {
		panic(err)
	}
	e.Regtest[coldStorageContainerName].NodeAPI = csNode
	nodeOutput, err := sendRequestToNodeWithBackoff(csNode, "getnewaddress", nil)
	if err != nil {
		panic(err)
	}
	return string(nodeOutput[1 : len(nodeOutput)-1])
}
