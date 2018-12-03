package main

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/api"
)

func init() {
	var startSeq int

	var cmdGetEvents = &cobra.Command{
		Use:   "get_events",
		Short: "Request events (optionally starting with given seq)",
		Run: func(cmd *cobra.Command, args []string) {
			seqRequest, err := json.Marshal(api.SubscribeMessage{startSeq})

			if err != nil {
				log.Fatal(err)
			}

			resp, err := http.Post(
				urljoin(apiURL, "/get_events"),
				"application/json",
				bytes.NewBuffer([]byte(seqRequest)),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			showResponse(resp.Body)
		},
	}

	cmdGetEvents.Flags().IntVarP(&startSeq, "seq", "s", 0, "sequence number to send events from")

	cli.AddCommand(cmdGetEvents)
}
