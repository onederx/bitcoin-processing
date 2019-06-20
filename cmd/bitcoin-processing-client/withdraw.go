package main

import (
	"encoding/json"
	"log"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
)

func init() {
	var withdrawID string
	var withdrawFeeType string
	var withdrawMetainfoString string

	makeWithdrawCommmandRunner := func(url string, toColdStorage bool) func(cmd *cobra.Command, args []string) {
		return func(cmd *cobra.Command, args []string) {
			var withdrawMetainfo interface{}
			var address, amountStr, feeStr string

			withdrawIDParsed, _ := uuid.FromString(withdrawID)

			if len(args) == 3 {
				address, amountStr, feeStr = args[0], args[1], args[2]
			} else {
				if !toColdStorage {
					panic("Regular withdraw requires 3 args")
				}
				amountStr, feeStr = args[0], args[1]
			}

			amount, err := bitcoin.BTCAmountFromStringedFloat(amountStr)

			if err != nil {
				log.Fatalf(
					"Failed to convert given amount value %q to bitcoin amount",
					amountStr)
			}

			fee, err := bitcoin.BTCAmountFromStringedFloat(feeStr)

			if err != nil {
				log.Fatalf(
					"Failed to convert given fee value %q to bitcoin amount",
					feeStr)
			}

			var requestData = wallet.WithdrawRequest{
				ID:      withdrawIDParsed,
				Address: address,
				Amount:  amount,
				Fee:     fee,
				FeeType: withdrawFeeType,
			}
			if withdrawMetainfoString != "" {
				err := json.Unmarshal(
					[]byte(withdrawMetainfoString),
					&withdrawMetainfo,
				)
				if err != nil {
					log.Fatalf(
						"Checking that metainfo is a valid JSON failed: %s",
						err,
					)
				}
				requestData.Metainfo = withdrawMetainfo
			}

			cli := client.NewClient(apiURL)

			if toColdStorage {
				showResponse(cli.WithdrawToColdStorage(&requestData))
			} else {
				showResponse(cli.Withdraw(&requestData))
			}
		}
	}

	var cmdWithdraw = &cobra.Command{
		Use:     "withdraw ADDRESS AMOUNT FEE",
		Example: "withdraw mv4rnyY3Su5gjcDNzbMLKBQkBicCtHUtFB 0.3 0.002",
		Short:   "Withdraw money to bitcoin address",
		Args:    cobra.ExactArgs(3),
		Run:     makeWithdrawCommmandRunner("/withdraw", false),
	}

	var cmdWithdrawToColdStorage = &cobra.Command{
		Use:     "withdraw_to_cold_storage [ADDRESS] AMOUNT FEE",
		Example: "withdraw_to_cold_storage mv4rnyY3Su5gjcDNzbMLKBQkBicCtHUtFB 0.3 0.002",
		Short:   "Withdraw money from processing wallet to cold storage",
		Args:    cobra.RangeArgs(2, 3),
		Run:     makeWithdrawCommmandRunner("/withdraw_to_cold_storage", true),
	}

	for _, cmd := range []*cobra.Command{cmdWithdraw, cmdWithdrawToColdStorage} {
		cmd.Flags().StringVarP(&withdrawID, "id", "i", "", "id of withdraw transaction")
		cmd.Flags().StringVarP(&withdrawFeeType, "fee-type", "t", "", "transaction fee type")
		cmd.Flags().StringVarP(&withdrawMetainfoString, "metainfo", "m", "", "metainfo to attach to withdraw")
		cli.AddCommand(cmd)
	}
}
