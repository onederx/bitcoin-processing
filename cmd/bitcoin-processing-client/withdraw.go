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

	var cmdWithdraw = &cobra.Command{
		Use:     "withdraw ADDRESS AMOUNT FEE",
		Example: "withdraw mv4rnyY3Su5gjcDNzbMLKBQkBicCtHUtFB 0.3",
		Short:   "Withdraw money to bitcoin address",
		Args:    cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			var withdrawMetainfo interface{}

			withdrawIdParsed, _ := uuid.FromString(withdrawId)
			var requestData = api.WithdrawRequest{
				Id:      withdrawIdParsed,
				Address: args[0],
				Amount:  args[1],
				Fee:     args[2],
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
				urljoin(apiURL, "/withdraw"),
				"application/json",
				bytes.NewBuffer(requestBody),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
		},
	}

	cmdWithdraw.Flags().StringVarP(&withdrawId, "id", "i", "", "id of withdraw transaction")
	cmdWithdraw.Flags().StringVarP(&withdrawFeeType, "fee-type", "t", "", "transaction fee type")
	cmdWithdraw.Flags().StringVarP(&withdrawMetainfoString, "metainfo", "m", "", "metainfo to attach to withdraw")

	cli.AddCommand(cmdWithdraw)
}
