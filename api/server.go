package api

import (
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

// Server runs http and websocket servers providing API. All user interaction
// with processing app goes through it
type Server struct {
	wallet        *wallet.Wallet
	eventBroker   *events.EventBroker
	listenAddress string
	httpServer    *http.Server
}

// NewServer creates new instance of API server
func NewServer(listenAddress string, btcWallet *wallet.Wallet, eventBroker *events.EventBroker) *Server {
	httpServer := &http.Server{
		Addr:    listenAddress,
		Handler: http.NewServeMux(),
	}
	server := &Server{
		wallet:        btcWallet,
		eventBroker:   eventBroker,
		listenAddress: listenAddress,
		httpServer:    httpServer,
	}
	server.initHTTPAPIServer()
	server.initWebsocketAPIServer()
	return server
}

// Run starts HTTP and websocket server
func (s *Server) Run() {
	log.Printf("Starting API server on %s", s.listenAddress)
	log.Fatal(s.httpServer.ListenAndServe())
}
