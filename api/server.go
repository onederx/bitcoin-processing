package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/wallet"
)

const shutdownTimeout = 150 * time.Millisecond

// Server runs http and websocket servers providing API. All user interaction
// with processing app goes through it
type Server struct {
	wallet                   *wallet.Wallet
	eventBroker              events.EventBroker
	listenAddress            string
	allowWithdrawalWithoutID bool
	httpServer               *http.Server
}

// NewServer creates new instance of API server
func NewServer(listenAddress string, btcWallet *wallet.Wallet, eventBroker events.EventBroker, allowWithdrawalWithoutID bool) *Server {
	httpServer := &http.Server{
		Addr:    listenAddress,
		Handler: http.NewServeMux(),
	}
	server := &Server{
		wallet:                   btcWallet,
		eventBroker:              eventBroker,
		listenAddress:            listenAddress,
		allowWithdrawalWithoutID: allowWithdrawalWithoutID,
		httpServer:               httpServer,
	}
	server.initHTTPAPIServer()
	server.initWebsocketAPIServer()
	return server
}

// Run starts HTTP and websocket server
func (s *Server) Run() error {
	log.Printf("Starting API server on %s", s.listenAddress)
	err := s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Stop() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	s.httpServer.Shutdown(shutdownCtx)
}
