package main

import (
	"encoding/json"
	"log"

	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func init() {
	var newWalletMetainfoString string

	var cmdNewWallet = &cobra.Command{
		Use:   "new_wallet",
		Short: "Create new wallet",
		Run: func(cmd *cobra.Command, args []string) {
			var newWalletMetainfo interface{}
			if newWalletMetainfoString != "" {
				err := json.Unmarshal(
					[]byte(newWalletMetainfoString),
					&newWalletMetainfo,
				)
				if err != nil {
					log.Fatalf(
						"Checking that metainfo is a valid JSON failed: %s",
						err,
					)
				}
			}
			showResponse(client.NewClient(apiURL).NewWallet(newWalletMetainfo))
		},
	}

	cmdNewWallet.Flags().StringVarP(&newWalletMetainfoString, "metainfo", "m", "", "wallet metainfo")

	cli.AddCommand(cmdNewWallet)
}
