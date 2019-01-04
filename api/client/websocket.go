package client

import (
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/events"
)

type WebsocketClient struct {
	Done <-chan struct{}

	interrupt chan<- struct{}

	owner *Client
}

func (cli *Client) NewWebsocketClient(startSeq int, messageCb func(*events.NotificationWithSeq)) (*WebsocketClient, error) {
	u, err := url.Parse(cli.apiBaseURL)

	if err != nil {
		return nil, err
	}

	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.Path = "/ws"

	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	interrupt := make(chan struct{}, 1)
	done := make(chan struct{})
	clientFinished := make(chan struct{})

	go func() {
		defer close(done)
		for {
			var notification events.NotificationWithSeq
			err := c.ReadJSON(&notification)
			if err != nil {
				log.Println("read:", err)
				return
			}
			messageCb(&notification)
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

	wsClient := &WebsocketClient{
		Done:      clientFinished,
		interrupt: interrupt,
		owner:     cli,
	}

	cli.websocketClients = append(cli.websocketClients, wsClient)

	return wsClient, nil
}

func (w *WebsocketClient) Close() {
	if w.owner == nil {
		log.Println("Close called on already closed websocket client")
		return
	}
	for i, wsClient := range w.owner.websocketClients {
		if w == wsClient {
			w.owner.websocketClients = append(w.owner.websocketClients[:i], w.owner.websocketClients[i+1:]...)
			break
		}
	}
	w.owner = nil
	close(w.interrupt)
	<-w.Done
}
