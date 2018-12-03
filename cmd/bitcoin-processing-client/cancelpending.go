package main

import (
	"bytes"
	"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func init() {
	var cmdCancelPending = &cobra.Command{
		Use:     "cancel_pending TX_ID",
		Example: "cancel_pending aec79cbf-79c4-46ef-a54f-63a0cf451fe2",
		Short:   "Cancel pending transaction",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			txID, err := uuid.FromString(args[0])
			if err != nil {
				log.Fatal(err)
			}
			txIDJSON, err := json.Marshal(txID)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := http.Post(
				urljoin(apiURL, "/cancel_pending"),
				"application/json",
				bytes.NewBuffer(txIDJSON),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
		},
	}

	cli.AddCommand(cmdCancelPending)
}
