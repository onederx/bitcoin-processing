package main

import (
	"bytes"
	"encoding/json"
	"github.com/onederx/bitcoin-processing/api"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func init() {
	var withdrawId string
	var withdrawFeeType string
	var withdrawMetainfoString string

	makeWithdrawCommmandRunner := func(url string, toColdStorage bool) func(cmd *cobra.Command, args []string) {
		return func(cmd *cobra.Command, args []string) {
			var withdrawMetainfo interface{}
			var address, amount, fee string

			withdrawIdParsed, _ := uuid.FromString(withdrawId)

			if len(args) == 3 {
				address, amount, fee = args[0], args[1], args[2]
			} else {
				if !toColdStorage {
					panic("Regular withdraw requires 3 args")
				}
				amount, fee = args[0], args[1]
			}

			var requestData = api.WithdrawRequest{
				Id:      withdrawIdParsed,
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
					log.Fatal(err)
				}
				requestData.Metainfo = withdrawMetainfo
			}
			requestBody, err := json.Marshal(requestData)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := http.Post(
				urljoin(apiURL, url),
				"application/json",
				bytes.NewBuffer(requestBody),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
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
		cmd.Flags().StringVarP(&withdrawId, "id", "i", "", "id of withdraw transaction")
		cmd.Flags().StringVarP(&withdrawFeeType, "fee-type", "t", "", "transaction fee type")
		cmd.Flags().StringVarP(&withdrawMetainfoString, "metainfo", "m", "", "metainfo to attach to withdraw")
		cli.AddCommand(cmd)
	}
}
