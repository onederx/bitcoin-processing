package integrationtests

import (
	"fmt"
	"log"
	"time"

	"github.com/onederx/bitcoin-processing/api/client"
)

type websocketListener struct {
	interrupt chan<- struct{}
	done      <-chan struct{}
	stopped   bool

	messages chan []byte
}

func (e *testEnvironment) newWebsocketListener(startSeq int) (*websocketListener, error) {
	var err error
	log.Println("Starting websocket listener")
	apiUrl := fmt.Sprintf("http://%s:8000", e.processing.ip)
	listener := &websocketListener{
		messages: make(chan []byte, listenersMessageQueueSize),
	}
	listener.interrupt, listener.done, err = client.NewWebsocketClient(
		apiUrl, startSeq, listener.processMessage,
	)
	if err != nil {
		log.Printf("Websocket listener start failed: %s", err)
		return nil, err
	}
	e.websocketListeners = append(e.websocketListeners, listener)
	log.Println("Websocket listener started")
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
	l.stopped = true
	log.Printf("Stopping websocket listener")
	l.interrupt <- struct{}{}
	<-l.done
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
