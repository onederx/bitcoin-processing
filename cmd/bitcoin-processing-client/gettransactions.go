package main

import (
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/wallet"
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
			cli := client.NewClient(apiURL)
			showResponse(cli.GetTransactions(&filter))
		},
	}

	cmdGetTransactions.Flags().StringVarP(&directionFilter, "direction", "d", "", "tx direction filter")
	cmdGetTransactions.Flags().StringVarP(&statusFilter, "status", "s", "", "tx status filter")

	cli.AddCommand(cmdGetTransactions)
}
