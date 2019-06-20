package testenv

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

	"github.com/onederx/bitcoin-processing/wallet"
)

type callbackRequest struct {
	Method string
	URL    *url.URL
	body   []byte
}

func (c *callbackRequest) unmarshal() (*wallet.TxNotification, error) {
	var notification wallet.TxNotification
	err := json.Unmarshal(c.body, &notification)
	return &notification, err
}

func (c *callbackRequest) UnmarshalOrFail(t *testing.T) *wallet.TxNotification {
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

func (e *TestEnvironment) startCallbackListener() {
	log.Println("Starting callback listener server")
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.CallbackHandler != nil {
			e.CallbackHandler(w, r)
			return
		}
		var err error
		cbRequest := callbackRequest{
			Method: r.Method,
			URL:    r.URL,
		}
		cbRequest.body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			panic(fmt.Sprintf("Failed to read body of request that arrived to HTTP callback: %v", err))
		}
		e.callbackMessageQueue <- &cbRequest
	}))
	server.Listener = newTestRandomFreePortListener(e.NetworkGateway)
	server.Start()
	e.CallbackURL = server.URL + DefaultCallbackURLPath
	e.callbackListener = server
	e.callbackMessageQueue = make(chan *callbackRequest, listenersMessageQueueSize)
	log.Printf("Callback listener server started on %s", server.URL)
}

func (e *TestEnvironment) stopCallbackListener() {
	log.Println("Stopping callback listener")
	e.callbackListener.Close()
	close(e.callbackMessageQueue)
	log.Println("Callback listener stopped")
}

func (e *TestEnvironment) GetNextCallbackRequestWithTimeout(t *testing.T) *callbackRequest {
	select {
	case req := <-e.callbackMessageQueue:
		return req
	case <-time.After(listenersMessageWaitTimeout):
		t.Fatal("No message arrived before timeout")
	}
	return nil
}

func (e *TestEnvironment) GetNextCallbackNotificationWithTimeout(t *testing.T) *wallet.TxNotification {
	req := e.GetNextCallbackRequestWithTimeout(t)
	return req.UnmarshalOrFail(t)
}
