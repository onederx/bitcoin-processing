package integrationtests

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
)

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
		fmt.Fprintln(w, "Hello, client")
	}))
	server.Listener = newTestRandomFreePortListener(e.networkGateway)
	server.Start()
	e.callbackURL = server.URL
	e.callbackListener = server
	log.Printf("Callback listener server started on %s", server.URL)
}

func (e *testEnvironment) stopCallbackListener() {
	log.Println("Stopping callback listener")
	e.callbackListener.Close()
	log.Println("Callback listener stopped")
}
