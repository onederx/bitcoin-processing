package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

func (s *APIServer) newBitcoinAddress(response http.ResponseWriter, request *http.Request) {
	var metainfo map[string]interface{}
	var body, responseBody []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &metainfo); err != nil {
		panic(err)
	}
	account, err := s.wallet.CreateAccount(metainfo)
	if err != nil {
		panic(err)
	}
	if responseBody, err = json.Marshal(account); err != nil {
		panic(err)
	}
	s.eventBroker.Notify(events.NewAddressEvent, account)
	response.Write(responseBody)
}

func (s *APIServer) notifyWalletTxStatusChanged(response http.ResponseWriter, request *http.Request) {
	s.eventBroker.Notify(events.CheckTxStatusEvent, "")
}

func (s *APIServer) withdraw(response http.ResponseWriter, request *http.Request) {
	var req wallet.WithdrawRequest
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &req); err != nil {
		panic(err)
	}
	if err = s.wallet.Withdraw(&req); err != nil {
		panic(err)
	}
}

func (s *APIServer) getHotStorageAddress(response http.ResponseWriter, request *http.Request) {
	response.Write([]byte(s.wallet.GetHotWalletAddress() + "\n"))
}

func (s *APIServer) getTransactions(response http.ResponseWriter, request *http.Request) {
	var txFilter struct {
		Direction string
		Status    string
	}
	var body, responseBody []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		panic(err)
	}
	if len(body) > 0 {
		if err = json.Unmarshal(body, &txFilter); err != nil {
			panic(err)
		}
	}
	txns, err := s.wallet.GetTransactionsWithFilter(txFilter.Direction, txFilter.Status)
	if err != nil {
		panic(err)
	}
	if responseBody, err = json.Marshal(txns); err != nil {
		panic(err)
	}
	response.Write(responseBody)
}

func (s *APIServer) handle(urlPattern, method string, handler func(http.ResponseWriter, *http.Request)) {
	requestDispatcher := s.httpServer.Handler.(*http.ServeMux)
	requestDispatcher.HandleFunc(urlPattern, func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Caught error handling '%s' %s", urlPattern, err)
				response.WriteHeader(500)
				fmt.Fprintf(response, "Error: %s\n", err)
			}
		}()
		if method != "" && method != request.Method {
			response.WriteHeader(405)
			fmt.Fprintf(response, "Method %s is not allowed", request.Method)
		}
		handler(response, request)
	})
}

func (s *APIServer) initHTTPAPIServer() {
	s.handle("/new-address", "", s.newBitcoinAddress)
	s.handle("/notify-wallet", "", s.notifyWalletTxStatusChanged)
	s.handle("/withdraw", "", s.withdraw)
	s.handle("/get-hot-storage-address", "", s.getHotStorageAddress)
	s.handle("/get-transactions", "", s.getTransactions)
}
