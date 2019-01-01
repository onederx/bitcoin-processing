package client

import (
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/onederx/bitcoin-processing/api"
)

func NewWebsocketClient(apiURL string, startSeq int, messageCb func([]byte)) (chan<- struct{}, <-chan struct{}, error) {
	u, err := url.Parse(apiURL)

	if err != nil {
		return nil, nil, err
	}

	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.Path = "/ws"

	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	interrupt := make(chan struct{}, 1)
	done := make(chan struct{})
	clientFinished := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			messageCb(message)
		}
	}()

	go func() {
		defer c.Close()
		defer close(clientFinished)
		err := c.WriteJSON(api.SubscribeMessage{Seq: startSeq})
		if err != nil {
			log.Println("write subscribe message:", err)
		}
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
	}()
	return interrupt, clientFinished, nil
}
