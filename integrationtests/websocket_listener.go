package integrationtests

import (
	"log"
	"testing"
	"time"

	"github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/events"
)

type websocketListener struct {
	wsClient *client.WebsocketClient
	stopped  bool

	messages chan *events.NotificationWithSeq

	lastSeq int
}

func (e *testEnvironment) newWebsocketListener(startSeq int) (*websocketListener, error) {
	listener := &websocketListener{
		messages: make(chan *events.NotificationWithSeq, listenersMessageQueueSize),
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

func (l *websocketListener) processMessage(message *events.NotificationWithSeq) {
	l.lastSeq = message.Seq
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

func (l *websocketListener) getNextMessageWithTimeout(t *testing.T) *events.NotificationWithSeq {
	select {
	case msg := <-l.messages:
		return msg
	case <-time.After(listenersMessageWaitTimeout):
		t.Fatal("No message arrived before timeout")
	}
	return nil
}
