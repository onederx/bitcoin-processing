package main

import (
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func runWalletInfoCommand(cmd *cobra.Command, args []string) {
	cli := client.NewClient(apiURL)

	switch cmd.Use {
	case "get_hot_storage_address":
		showResponse(cli.GetHotStorageAddress())
	case "get_balance":
		showResponse(cli.GetBalance())
	case "get_required_from_cold_storage":
		showResponse(cli.GetRequiredFromColdStorage())
	default:
		panic("Unknown command " + cmd.Use)
	}
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
