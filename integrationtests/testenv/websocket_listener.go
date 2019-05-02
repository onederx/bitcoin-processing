package testenv

import (
	"log"
	"testing"
	"time"

	"github.com/onederx/bitcoin-processing/api/client"
	"github.com/onederx/bitcoin-processing/events"
)

type WebsocketListener struct {
	wsClient *client.WebsocketClient
	stopped  bool

	messages chan *events.NotificationWithSeq

	LastSeq int
}

func (e *TestEnvironment) NewWebsocketListener(startSeq int) (*WebsocketListener, error) {
	listener := &WebsocketListener{
		messages: make(chan *events.NotificationWithSeq, listenersMessageQueueSize),
	}
	wsClient, err := e.ProcessingClient.NewWebsocketClient(
		startSeq, listener.processMessage,
	)
	if err != nil {
		return nil, err
	}
	listener.wsClient = wsClient
	e.WebsocketListeners = append(e.WebsocketListeners, listener)
	return listener, nil
}

func (l *WebsocketListener) processMessage(message *events.NotificationWithSeq) {
	l.LastSeq = message.Seq
	l.messages <- message
}

func (l *WebsocketListener) Stop() {
	if l.stopped {
		log.Println("Websocket listener stop called on already stopped listener")
		return
	}
	l.wsClient.Close()
	l.wsClient = nil
	close(l.messages)
	log.Printf("Websocket listener stopped")
}

func (l *WebsocketListener) GetNextMessageWithTimeout(t *testing.T) *events.NotificationWithSeq {
	select {
	case msg := <-l.messages:
		return msg
	case <-time.After(listenersMessageWaitTimeout):
		t.Fatal("No message arrived before timeout")
	}
	return nil
}
