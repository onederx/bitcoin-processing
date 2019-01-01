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
			interrupt, done, err := client.NewWebsocketClient(apiURL, startSeq, func(message []byte) {
				log.Printf("recv: %s", message)
			})

			if err != nil {
				log.Fatal(err)
			}

			sigInt := make(chan os.Signal, 1)
			signal.Notify(sigInt, os.Interrupt)

			go func() {
				select {
				case <-sigInt:
					interrupt <- struct{}{}
				case <-done:
				}
			}()
			<-done
		},
	}

	cmdWebsocket.Flags().IntVarP(&startSeq, "seq", "s", 0, "sequence number to send messages from")

	cli.AddCommand(cmdWebsocket)
}
