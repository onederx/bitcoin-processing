package api

import (
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

type APIServer struct {
	wallet        *wallet.Wallet
	eventBroker   *events.EventBroker
	listenAddress string
	httpServer    *http.Server
}

func NewAPIServer(listenAddress string, btcWallet *wallet.Wallet, eventBroker *events.EventBroker) *APIServer {
	httpServer := &http.Server{
		Addr:    listenAddress,
		Handler: http.NewServeMux(),
	}
	server := &APIServer{
		wallet:        btcWallet,
		eventBroker:   eventBroker,
		listenAddress: listenAddress,
		httpServer:    httpServer,
	}
	server.initHTTPAPIServer()
	server.initWebsocketAPIServer()
	return server
}

func (s *APIServer) Run() {
	log.Printf("Starting API server on %s", s.listenAddress)
	log.Fatal(s.httpServer.ListenAndServe())
}
