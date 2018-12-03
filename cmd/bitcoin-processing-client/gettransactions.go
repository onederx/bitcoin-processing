package main

import (
	"bytes"
	"encoding/json"
	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func init() {
	var directionFilter string
	var statusFilter string

	var cmdGetTransactions = &cobra.Command{
		Use:   "get_transactions",
		Short: "Get list of transactions, optionally filtered by status or direction",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if directionFilter != "" {
				_, err := wallet.TransactionDirectionFromString(directionFilter)
				if err != nil {
					return err
				}
			}
			if statusFilter != "" {
				_, err := wallet.TransactionStatusFromString(statusFilter)
				if err != nil {
					return err
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			filter := api.GetTransactionsFilter{
				Direction: directionFilter,
				Status:    statusFilter,
			}
			filterJSON, err := json.Marshal(filter)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := http.Post(
				urljoin(apiURL, "/get_transactions"),
				"application/json",
				bytes.NewBuffer(filterJSON),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
		},
	}

	cmdGetTransactions.Flags().StringVarP(&directionFilter, "direction", "d", "", "tx direction filter")
	cmdGetTransactions.Flags().StringVarP(&statusFilter, "status", "s", "", "tx status filter")

	cli.AddCommand(cmdGetTransactions)
}
