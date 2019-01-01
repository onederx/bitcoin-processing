package integrationtests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

type callbackRequest struct {
	method string
	url    *url.URL
	body   []byte
}

func (c *callbackRequest) unmarshal() (*wallet.TxNotification, error) {
	var notification wallet.TxNotification
	err := json.Unmarshal(c.body, &notification)
	return &notification, err
}

func newTestRandomFreePortListener(host string) net.Listener {
	l, err := net.Listen("tcp", fmt.Sprintf("%s:0", host))
	if err != nil {
		panic(fmt.Sprintf("Failed to pick up random free port %v", err))
	}
	return l
}

func (e *testEnvironment) startCallbackListener() {
	log.Println("Starting callback listener server")
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		cbRequest := callbackRequest{
			method: r.Method,
			url:    r.URL,
		}
		cbRequest.body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			panic(fmt.Sprintf("Failed to read body of request that arrived to HTTP callback: %v", err))
		}
		e.callbackMessageQueue <- &cbRequest
	}))
	server.Listener = newTestRandomFreePortListener(e.networkGateway)
	server.Start()
	e.callbackURL = server.URL + defaultCallbackURLPath
	e.callbackListener = server
	e.callbackMessageQueue = make(chan *callbackRequest, listenersMessageQueueSize)
	log.Printf("Callback listener server started on %s", server.URL)
}

func (e *testEnvironment) stopCallbackListener() {
	log.Println("Stopping callback listener")
	e.callbackListener.Close()
	close(e.callbackMessageQueue)
	log.Println("Callback listener stopped")
}

func (e *testEnvironment) checkNextCallbackRequest(checker func(*callbackRequest)) {
	select {
	case req := <-e.callbackMessageQueue:
		checker(req)
	case <-time.After(listenersMessageWaitTimeout):
		panic("No message arrived before timeout")
	}
}
