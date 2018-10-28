package api

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/events"
)

type client struct {
	conn *websocket.Conn
	isOk bool
}

var upgrader = websocket.Upgrader{} // use default options
var clients []*client

func broadcastEventsToClients() {
	for event := range events.EventQueue {
		closedConnIndexes := make([]int, 0)
		for i, clientConn := range clients {
			if clientConn.conn == nil || !clientConn.isOk {
				closedConnIndexes = append(closedConnIndexes, i)
				continue
			}
			err := clientConn.conn.WriteMessage(websocket.TextMessage, event)
			if err != nil {
				log.Println("write:", err)

				clientConn.isOk = false
				closedConnIndexes = append(closedConnIndexes, i)
			}
		}
		for _, closedConnIndex := range closedConnIndexes {
			clients = append(
				clients[:closedConnIndex],
				clients[closedConnIndex+1:]...,
			)
		}
	}
}

func handleWebsocketConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade:", err)
		return
	}

	c := client{
		conn: conn,
		isOk: true,
	}
	clients = append(clients, &c)
	defer conn.Close()
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			c.isOk = false
			unexpected := websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseNormalClosure,
			)
			if unexpected {
				log.Println("read:", err)
			}
		}
		if !c.isOk {
			c.conn = nil
			break
		}
	}
}

func initWebsocketAPIServer() {
	http.HandleFunc("/ws", handleWebsocketConnection)

	go broadcastEventsToClients()
}
