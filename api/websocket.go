package api

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// SubscribeMessage simply holds one integer - a sequence number to send events
// from - and is used to describe JSON struct sent from client to request
// events starting with this sequence number
type SubscribeMessage struct {
	Seq int `json:"seq"`
}

var upgrader = websocket.Upgrader{} // use default options

func shutdownConnection(conn *websocket.Conn) {
	err := conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		log.Println("Closing connection with client:", err)
	}
}

func readSeqFromClient(conn *websocket.Conn) (SubscribeMessage, error) {
	var decodedMessage SubscribeMessage

	err := conn.ReadJSON(&decodedMessage)

	if err != nil {
		shutdownConnection(conn)
		return decodedMessage, err
	}
	return decodedMessage, nil
}

func (s *Server) handleWebsocketConnection(w http.ResponseWriter, r *http.Request) {
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

	eventSequenceQueue := s.eventBroker.SubscribeFromSeq(subscribeMessage.Seq)
	defer s.eventBroker.UnsubscribeFromSeq(eventSequenceQueue)

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
		case eventSequence, ok := <-eventSequenceQueue:
			if !ok {
				log.Printf(
					"Websocket event sender: event queue closed for some " +
						"reason. Closing connection.",
				)
				return
			}
			for _, event := range eventSequence {
				err = conn.WriteJSON(event)
				if err != nil {
					log.Println("write:", err)
					return
				}
			}
		}
	}
}

func (s *Server) initWebsocketAPIServer() {
	requestDispatcher := s.httpServer.Handler.(*http.ServeMux)
	requestDispatcher.HandleFunc("/ws", s.handleWebsocketConnection)
}
