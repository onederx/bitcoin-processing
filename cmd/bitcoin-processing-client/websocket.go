package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/onederx/bitcoin-processing/api"
)

func init() {
	var startSeq int

	var cmdWebsocket = &cobra.Command{
		Use:   "websocket",
		Short: "Subscribe to events via websocket",
		Run: func(cmd *cobra.Command, args []string) {
			seqRequest, err := json.Marshal(api.SubscribeMessage{startSeq})

			if err != nil {
				log.Fatal(err)
			}

			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, os.Interrupt)

			u, err := url.Parse(apiURL)

			if err != nil {
				log.Fatal(err)
			}

			u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
			u.Path = "/ws"

			log.Printf("Connecting to %s", u.String())

			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				log.Fatal("Dial:", err)
			}
			defer c.Close()

			done := make(chan struct{})
			c.WriteMessage(websocket.TextMessage, seqRequest)

			go func() {
				defer close(done)
				for {
					_, message, err := c.ReadMessage()
					if err != nil {
						log.Println("read:", err)
						return
					}
					log.Printf("recv: %s", message)
				}
			}()

			for {
				select {
				case <-done:
					return
				case <-interrupt:
					log.Println("interrupt")

					// Cleanly close the connection by sending a close message and then
					// waiting (with timeout) for the server to close the connection.
					err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("write close:", err)
						return
					}
					select {
					case <-done:
					case <-time.After(time.Second):
					}
					return
				}
			}
		},
	}

	cmdWebsocket.Flags().IntVarP(&startSeq, "seq", "s", 0, "sequence number to send messages from")

	cli.AddCommand(cmdWebsocket)
}
