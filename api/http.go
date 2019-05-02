package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

const (
	NewWalletURL                  = "/new_wallet"
	NotifyWalletURL               = "/notify_wallet"
	WithdrawURL                   = "/withdraw"
	GetHotStorageAddressURL       = "/get_hot_storage_address"
	GetTransactionsURL            = "/get_transactions"
	GetBalanceURL                 = "/get_balance"
	GetRequiredFromColdStorageURL = "/get_required_from_cold_storage"
	CancelPendingURL              = "/cancel_pending"
	WithdrawToColdStorageURL      = "/withdraw_to_cold_storage"
	ConfirmURL                    = "/confirm"
	GetEventsURL                  = "/get_events"
)

// GetTransactionsFilter describes data sent by client to set up filters in
// /get_transactions request. Currently, filtering by direction and status
// is supported, empty value means do not filter
type GetTransactionsFilter struct {
	Direction string `json:"direction,omitempty"`
	Status    string `json:"status,omitempty"`
}

type HTTPAPIResponseError string

func (err HTTPAPIResponseError) Error() string {
	return string(err)
}

type GenericHTTPAPIResponse struct {
	Error HTTPAPIResponseError `json:"error"`
}

type BalanceInfo struct {
	Balance           bitcoin.BTCAmount `json:"balance"`
	BalanceWithUnconf bitcoin.BTCAmount `json:"balance_including_unconfirmed"`
}

type httpAPIResponse struct {
	GenericHTTPAPIResponse
	Result interface{} `json:"result"`
}

func (s *Server) respond(response http.ResponseWriter, data interface{}, err error) {
	var responseBody []byte
	if err != nil {
		responseBody, err = json.Marshal(httpAPIResponse{
			GenericHTTPAPIResponse: GenericHTTPAPIResponse{
				Error: HTTPAPIResponseError(err.Error()),
			}},
		)
		if err != nil {
			panic("Failed to marshal error response for error " + err.Error())
		}
		_, err = response.Write(responseBody)
		if err != nil {
			panic(fmt.Sprintf(
				"Failed to write error response %q: %s",
				responseBody,
				err,
			))
		}
		return
	}
	responseBody, err = json.Marshal(httpAPIResponse{
		GenericHTTPAPIResponse: GenericHTTPAPIResponse{Error: "ok"},
		Result:                 data,
	})
	if err != nil {
		panic("Failed to marshal ok response for error " + err.Error())
	}
	_, err = response.Write(responseBody)
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to write ok response %q: %s",
			responseBody,
			err,
		))
	}
}

func (s *Server) newBitcoinAddress(response http.ResponseWriter, request *http.Request) {
	var metainfo map[string]interface{}
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if len(body) > 0 {
		if err = json.Unmarshal(body, &metainfo); err != nil {
			s.respond(response, nil, err)
			return
		}
	} else {
		metainfo = nil
	}
	account, err := s.wallet.CreateAccount(metainfo)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	s.eventBroker.Notify(events.NewAddressEvent, account)
	s.respond(response, account, nil)
}

func (s *Server) notifyWalletTxStatusChanged(response http.ResponseWriter, request *http.Request) {
	s.eventBroker.Notify(events.CheckTxStatusEvent, "")
	s.respond(response, nil, nil)
}

func (s *Server) withdraw(toColdStorage bool, response http.ResponseWriter, request *http.Request) {
	var req wallet.WithdrawRequest

	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		s.respond(response, nil, err)
		return
	}
	if req.ID == uuid.Nil {
		req.ID = uuid.Must(uuid.NewV4())
		log.Printf("Generated new withdrawal id %s", req.ID)
	}
	if req.FeeType == "" {
		log.Printf("Fee type not specified: setting to 'fixed' by default")
		req.FeeType = "fixed"
	}
	if err := s.wallet.Withdraw(&req, toColdStorage); err != nil {
		s.respond(response, nil, err)
		return
	}
	s.respond(response, req, nil)
}

func (s *Server) withdrawRegular(response http.ResponseWriter, request *http.Request) {
	s.withdraw(false, response, request)
}

func (s *Server) withdrawToColdStorage(response http.ResponseWriter, request *http.Request) {
	s.withdraw(true, response, request)
}

func (s *Server) getHotStorageAddress(response http.ResponseWriter, request *http.Request) {
	s.respond(response, s.wallet.GetHotWalletAddress(), nil)
}

func (s *Server) getTransactions(response http.ResponseWriter, request *http.Request) {
	var txFilter GetTransactionsFilter
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if len(body) > 0 {
		if err = json.Unmarshal(body, &txFilter); err != nil {
			s.respond(response, nil, err)
			return
		}
	}
	txns, err := s.wallet.GetTransactionsWithFilter(txFilter.Direction, txFilter.Status)

	s.respond(response, txns, err)
}

func (s *Server) getBalance(response http.ResponseWriter, request *http.Request) {
	var respData BalanceInfo
	var err error
	respData.Balance, respData.BalanceWithUnconf, err = s.wallet.GetBalance()

	s.respond(response, respData, err)
}

func (s *Server) getRequiredFromColdStorage(response http.ResponseWriter, request *http.Request) {
	s.respond(response, s.wallet.GetMoneyRequiredFromColdStorage(), nil)
}

func (s *Server) cancelPending(response http.ResponseWriter, request *http.Request) {
	var id uuid.UUID
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if err = json.Unmarshal(body, &id); err != nil {
		s.respond(response, nil, err)
		return
	}
	err = s.wallet.CancelPendingTx(id)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	s.respond(response, nil, nil)
}

func (s *Server) confirmPendingTransaction(response http.ResponseWriter, request *http.Request) {
	var id uuid.UUID
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if err = json.Unmarshal(body, &id); err != nil {
		s.respond(response, nil, err)
		return
	}
	err = s.wallet.ConfirmPendingTransaction(id)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	s.respond(response, nil, nil)
}

func (s *Server) getEvents(response http.ResponseWriter, request *http.Request) {
	var body []byte
	var err error
	var seq int
	var subscription SubscribeMessage

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if len(body) > 0 {
		if err = json.Unmarshal(body, &subscription); err != nil {
			s.respond(response, nil, err)
			return
		}
		seq = subscription.Seq
	}
	events, err := s.eventBroker.GetEventsFromSeq(seq)

	if err != nil {
		s.respond(response, nil, err)
	}
	s.respond(response, events, nil)
}

func (s *Server) initHTTPAPIServer() {
	m := s.httpServer.Handler.(*http.ServeMux)
	m.HandleFunc(NewWalletURL, s.newBitcoinAddress)
	m.HandleFunc(NotifyWalletURL, s.notifyWalletTxStatusChanged)
	m.HandleFunc(WithdrawURL, s.withdrawRegular)
	m.HandleFunc(GetHotStorageAddressURL, s.getHotStorageAddress)
	m.HandleFunc(GetTransactionsURL, s.getTransactions)
	m.HandleFunc(GetBalanceURL, s.getBalance)
	m.HandleFunc(GetRequiredFromColdStorageURL, s.getRequiredFromColdStorage)
	m.HandleFunc(CancelPendingURL, s.cancelPending)
	m.HandleFunc(WithdrawToColdStorageURL, s.withdrawToColdStorage)
	m.HandleFunc(ConfirmURL, s.confirmPendingTransaction)
	m.HandleFunc(GetEventsURL, s.getEvents)
}
