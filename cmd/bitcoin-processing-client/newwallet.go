package main

import (
	"bytes"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func init() {
	var newWalletMetainfo string

	var cmdNewWallet = &cobra.Command{
		Use:   "new_wallet",
		Short: "Create new wallet",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Post(
				urljoin(apiURL, "/new_wallet"),
				"application/json",
				bytes.NewBuffer([]byte(newWalletMetainfo)),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
		},
	}

	cmdNewWallet.Flags().StringVarP(&newWalletMetainfo, "metainfo", "i", "", "wallet metainfo")

	cli.AddCommand(cmdNewWallet)
}
