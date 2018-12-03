package main

import (
	"bytes"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func runWalletInfoCommand(cmd *cobra.Command, args []string) {
	resp, err := http.Post(
		urljoin(apiURL, "/"+cmd.Use),
		"application/json",
		bytes.NewBuffer(nil),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	showResponse(resp.Body)

}

func init() {
	var cmd *cobra.Command

	commands := map[string]string{
		"get_hot_storage_address":        "Get address of hot storage",
		"get_balance":                    "Get current confirmed and unconfirmed balance",
		"get_required_from_cold_storage": "Get amount of money required to transfer from cold storage to fund all pending txns",
	}
	for command, description := range commands {
		cmd = &cobra.Command{
			Use:   command,
			Short: description,
			Run:   runWalletInfoCommand,
		}
		cli.AddCommand(cmd)
	}
}
