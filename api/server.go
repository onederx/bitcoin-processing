package api

import (
	"log"
	"net/http"
)

func RunAPIServer(listenAddress string) {
	log.Printf("Starting API server on %s", listenAddress)

	initHTTPAPIServer()
	initWebsocketAPIServer()

	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
