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
	"testing"
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

func (c *callbackRequest) unmarshalOrFail(t *testing.T) *wallet.TxNotification {
	notification, err := c.unmarshal()
	if err != nil {
		t.Fatalf("Failed to deserialize notification data from http "+
			"callback request body: %v", err)
	}
	return notification
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
		if e.callbackHandler != nil {
			e.callbackHandler(w, r)
			return
		}
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

func (e *testEnvironment) getNextCallbackRequestWithTimeout(t *testing.T) *callbackRequest {
	select {
	case req := <-e.callbackMessageQueue:
		return req
	case <-time.After(listenersMessageWaitTimeout):
		t.Fatal("No message arrived before timeout")
	}
	return nil
}

func (e *testEnvironment) getNextCallbackNotificationWithTimeout(t *testing.T) *wallet.TxNotification {
	req := e.getNextCallbackRequestWithTimeout(t)
	return req.unmarshalOrFail(t)
}
