package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func init() {
	var startSeq int

	var cmdWebsocket = &cobra.Command{
		Use:   "websocket",
		Short: "Subscribe to events via websocket",
		Run: func(cmd *cobra.Command, args []string) {
			cli := client.NewClient(apiURL)
			wsClient, err := cli.NewWebsocketClient(startSeq, func(message []byte) {
				log.Printf("recv: %s", message)
			})

			if err != nil {
				log.Fatal(err)
			}

			defer wsClient.Close()

			sigInt := make(chan os.Signal, 1)
			signal.Notify(sigInt, os.Interrupt)

			select {
			case <-wsClient.Done:
			case <-sigInt:
			}
		},
	}

	cmdWebsocket.Flags().IntVarP(&startSeq, "seq", "s", 0, "sequence number to send messages from")

	cli.AddCommand(cmdWebsocket)
}
