package main

import (
	"log"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func init() {
	var cmdMuteEvents = &cobra.Command{
		Use:   "mute_events [TX_ID | current_problematic]",
		Short: "Mute events assosiated with a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if args[0] != "current_problematic" {
				_, err := uuid.FromString(args[0])
				if err != nil {
					log.Fatal(err)
				}
			}
			err := client.NewClient(apiURL).MuteEventsForTxID(args[0])

			if err != nil {
				log.Fatal(err)
			}
			log.Print("OK")
		},
	}

	cli.AddCommand(cmdMuteEvents)
}
