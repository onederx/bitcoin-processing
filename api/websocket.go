package api

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/events"
)

type client struct {
	conn *websocket.Conn
	isOk bool
}

type subscribeMessage struct {
	Seq int
}

var upgrader = websocket.Upgrader{} // use default options
var clients []*client

func shutdownConnection(conn *websocket.Conn) {
	err := conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		log.Println("Closing connection with client:", err)
	}
}

func readSeqFromClient(conn *websocket.Conn) (subscribeMessage, error) {
	var decodedMessage subscribeMessage

	messageType, message, err := conn.ReadMessage()

	if messageType != websocket.TextMessage {
		log.Printf("Unexpected type of subscribe message: %v", messageType)
		return decodedMessage, errors.New("Bad subscribe message type")
	}

	err = json.Unmarshal(message, &decodedMessage)

	if err != nil {
		log.Printf("Failed to decode message from client: %s", message)
		shutdownConnection(conn)
		return decodedMessage, err
	}
	return decodedMessage, nil
}

func handleWebsocketConnection(w http.ResponseWriter, r *http.Request) {
	log.Print("Got new websocket subscriber")
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Print("Upgrade:", err)
		return
	}

	defer conn.Close()

	subscribeMessage, err := readSeqFromClient(conn)

	if err != nil {
		return
	}

	log.Print("Subscriber requested messages from seq ", subscribeMessage.Seq)

	eventQueue := events.SubscribeFromSeq(subscribeMessage.Seq)
	defer events.Unsubscribe(eventQueue)

	clientClosedConnection := make(chan struct{})

	go func() {
		defer close(clientClosedConnection)
		for {
			// Even though we don't expect any more data from client, we need to
			// continue reading from connection to get and handle close message
			_, message, err := conn.ReadMessage()
			if err != nil {
				unexpected := websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseNormalClosure,
				)
				if unexpected {
					log.Println("Read from client:", err)
				}
				return
			}
			log.Printf("Client sent: %s", message)
		}
	}()

	for {
		select {
		case <-clientClosedConnection:
			return
		case event := <-eventQueue:
			marshaledEvent, err := json.Marshal(&event)
			if err != nil {
				log.Printf(
					"Error: could not json-encode notification for ws",
					err,
				)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, marshaledEvent)
			if err != nil {
				log.Println("write:", err)
				return
			}
		}

	}
}

func initWebsocketAPIServer() {
	http.HandleFunc("/ws", handleWebsocketConnection)
}
