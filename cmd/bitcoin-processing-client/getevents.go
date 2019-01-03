package main

import (
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func init() {
	var startSeq int

	var cmdGetEvents = &cobra.Command{
		Use:   "get_events",
		Short: "Request events (optionally starting with given seq)",
		Run: func(cmd *cobra.Command, args []string) {
			showResponse(client.NewClient(apiURL).GetEvents(startSeq))
		},
	}

	cmdGetEvents.Flags().IntVarP(&startSeq, "seq", "s", 0, "sequence number to send events from")

	cli.AddCommand(cmdGetEvents)
}
