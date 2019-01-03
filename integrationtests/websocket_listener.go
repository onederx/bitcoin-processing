package integrationtests

import (
	"log"
	"time"

	"github.com/onederx/bitcoin-processing/api/client"
)

type websocketListener struct {
	wsClient *client.WebsocketClient
	stopped  bool

	messages chan []byte
}

func (e *testEnvironment) newWebsocketListener(startSeq int) (*websocketListener, error) {
	listener := &websocketListener{
		messages: make(chan []byte, listenersMessageQueueSize),
	}
	wsClient, err := e.processingClient.NewWebsocketClient(
		startSeq, listener.processMessage,
	)
	if err != nil {
		return nil, err
	}
	listener.wsClient = wsClient
	e.websocketListeners = append(e.websocketListeners, listener)
	return listener, nil
}

func (l *websocketListener) processMessage(message []byte) {
	l.messages <- message
}

func (l *websocketListener) stop() {
	if l.stopped {
		log.Println("Websocket listener stop called on already stopped listener")
		return
	}
	l.wsClient.Close()
	l.wsClient = nil
	close(l.messages)
	log.Printf("Websocket listener stopped")
}

func (l *websocketListener) checkNextMessage(checker func([]byte)) {
	select {
	case msg := <-l.messages:
		checker(msg)
	case <-time.After(listenersMessageWaitTimeout):
		panic("No message arrived before timeout")
	}
}
