package main

import (
	"log"
	"math/rand"
	//"path"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/onederx/bitcoin-processing/wallet"
)

var (
	apiURLArg, apiURL string
	nProcs            uint

	accounts   []string
	accountsMu sync.Mutex

	clientAddresses   []string
	clientAddressesMu sync.Mutex
)

var serverSettings settings.Settings

var cli = &cobra.Command{
	Use:   "racer",
	Short: "Tool to find race conditions in bitcoin-processing",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		apiURL = serverSettings.GetString("api.http.address")
		if !strings.HasPrefix(apiURL, "http") {
			apiURL = "http://" + apiURL
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		for i := 0; i < int(nProcs); i++ {
			go race()
		}
		select {}
	},
}

func makeWallet(c *client.Client) string {
	testMetainfo := struct {
		Test string
	}{Test: "123"}
	w, err := c.NewWallet(&testMetainfo)
	if err != nil {
		log.Print(err)
		return ""
	} else {
		return w.Address
	}
}

func addAccountAddress(address string) {
	accountsMu.Lock()
	defer accountsMu.Unlock()
	accounts = append(accounts, address)
}

func makeAndStoreWallet(c *client.Client) {
	address := makeWallet(c)
	log.Printf("Created wallet %s", address)
	if address != "" {
		addAccountAddress(address)
	}
}

func getAccountAddress() string {
	accountsMu.Lock()
	defer accountsMu.Unlock()
	if len(accounts) == 0 {
		return ""
	}
	res := accounts[len(accounts)-1]
	accounts = accounts[:len(accounts)-1]
	return res
}

func getClientAddress() string {
	clientAddressesMu.Lock()
	defer clientAddressesMu.Unlock()
	if len(clientAddresses) == 0 {
		return ""
	}
	res := clientAddresses[len(clientAddresses)-1]
	clientAddresses = clientAddresses[:len(clientAddresses)-1]
	return res
}

func addClientAddress(address string) {
	clientAddressesMu.Lock()
	defer clientAddressesMu.Unlock()
	clientAddresses = append(clientAddresses, address)
}

func makeClientAddress() string {
	cmd := exec.Command("docker", "exec", "regtest_node3_1", "bitcoin-cli", "-regtest", "getnewaddress")

	out, err := cmd.Output()

	log.Printf("%s", out)
	if err != nil {
		log.Printf("Failed to deposit: %s %v", err.(*exec.ExitError).Stderr, err.(*exec.ExitError))
	}
	return strings.TrimSpace(string(out))
}

func makeWithdraw(c *client.Client) {
	addrWasCreated := false

	addr := getClientAddress()

	if addr == "" {
		addr = makeClientAddress()
		if addr == "" {
			log.Printf("Failed to create client address")
			return
		}
		addrWasCreated = true
	}
	amount, err := bitcoin.BTCAmountFromStringedFloat("0.24")

	if err != nil {
		log.Fatalf(
			"Failed to convert given amount value %q to bitcoin amount",
			"0.24")
	}

	fee, err := bitcoin.BTCAmountFromStringedFloat("0.0003")

	if err != nil {
		log.Fatalf(
			"Failed to convert given fee value %q to bitcoin amount",
			"0.0003")
	}
	var requestData = wallet.WithdrawRequest{
		Address: addr,
		Amount:  amount,
		Fee:     fee,
	}
	res, err := c.Withdraw(&requestData)
	if err != nil {
		log.Printf("error creating withdraw: %v", err)
	} else {
		log.Printf("%v", res)
	}
	if addrWasCreated {
		addClientAddress(addr)
	}
}

func makeDeposit(c *client.Client) {
	addrWasCreated := false
	accountAddr := getAccountAddress()
	if accountAddr == "" {
		accountAddr = makeWallet(c)
		if accountAddr == "" {
			log.Printf("Failed to make account for deposit")
			return
		}
		addrWasCreated = true
	}

	cmd := exec.Command("docker", "exec", "regtest_node3_1", "bitcoin-cli", "-regtest", "sendtoaddress", accountAddr, "0.4")

	out, err := cmd.Output()

	log.Printf("%s", out)
	if err != nil {
		log.Printf("Failed to deposit: %s %v", err.(*exec.ExitError).Stderr, err.(*exec.ExitError))
	}

	if addrWasCreated {
		addAccountAddress(accountAddr)
	}
}

func genBlocks(c *client.Client) {
	cmd := exec.Command("docker", "exec", "regtest_node3_1", "bitcoin-cli", "-regtest", "generate", "2")

	out, err := cmd.Output()

	log.Printf("%s", out)
	if err != nil {
		log.Printf("Failed to generate blocks: %s %v", err.(*exec.ExitError).Stderr, err.(*exec.ExitError))
	}
}

func race() {
	c := client.NewClient(apiURL)

	actions := []func(c *client.Client){
		makeAndStoreWallet,
		makeDeposit,
		makeWithdraw,
		genBlocks,
	}

	for {
		action := actions[rand.Intn(len(actions))]
		action(c)
	}
}

func main() {
	cobra.OnInitialize(func() {
		var err error

		if serverSettings, err = settings.NewSettings("", cli); err == nil {
			log.Printf(
				"Loaded config file %s, will try to use API address from it "+
					"if not given explicitly",
				serverSettings.ConfigFileUsed(),
			)
		}
		serverSettings.GetViper().BindPFlag("api.http.address", cli.PersistentFlags().Lookup("api-url"))
	})

	rand.Seed(time.Now().Unix())

	cli.PersistentFlags().StringVarP(&apiURLArg, "api-url", "u", "http://localhost:8000", "url of bitcoin-processing API")

	nCPU := runtime.NumCPU()

	cli.PersistentFlags().UintVarP(&nProcs, "num-procs", "n", uint(nCPU), "number of concurrent clients")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
