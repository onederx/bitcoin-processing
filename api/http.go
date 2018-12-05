package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

type WithdrawRequest struct {
	Id       uuid.UUID   `json:"id,omitempty"`
	Address  string      `json:"address,omitempty"`
	Amount   string      `json:"amount"`
	Fee      string      `json:"fee,omitempty"`
	FeeType  string      `json:"fee_type,omitempty"`
	Metainfo interface{} `json:"metainfo"`
}

type GetTransactionsFilter struct {
	Direction string `json:"direction,omitempty"`
	Status    string `json:"status,omitempty"`
}

type httpAPIResponse struct {
	Error  string      `json:"error"`
	Result interface{} `json:"result"`
}

func (s *APIServer) respond(response http.ResponseWriter, data interface{}, err error) {
	var responseBody []byte
	if err != nil {
		responseBody, err = json.Marshal(httpAPIResponse{Error: err.Error()})
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
	responseBody, err = json.Marshal(httpAPIResponse{Error: "ok", Result: data})
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

func (s *APIServer) newBitcoinAddress(response http.ResponseWriter, request *http.Request) {
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

func (s *APIServer) notifyWalletTxStatusChanged(response http.ResponseWriter, request *http.Request) {
	s.eventBroker.Notify(events.CheckTxStatusEvent, "")
	s.respond(response, nil, nil)
}

func (s *APIServer) withdraw(toColdStorage bool, response http.ResponseWriter, request *http.Request) {
	var req WithdrawRequest
	var withdrawReq wallet.WithdrawRequest
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		s.respond(response, nil, err)
		return
	}
	if err = json.Unmarshal(body, &req); err != nil {
		s.respond(response, nil, err)
		return
	}
	withdrawReq.Address = req.Address
	withdrawReq.Amount, err = bitcoin.BitcoinAmountFromStringedFloat(req.Amount)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	withdrawReq.Fee, err = bitcoin.BitcoinAmountFromStringedFloat(req.Fee)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	if req.Id == uuid.Nil {
		withdrawReq.Id = uuid.Must(uuid.NewV4())
		log.Printf("Generated new withdrawal id %s", withdrawReq.Id)
	} else {
		withdrawReq.Id = req.Id
	}
	if req.FeeType == "" {
		log.Printf("Fee type not specified: setting to 'fixed' by default")
		withdrawReq.FeeType = "fixed"
	} else {
		withdrawReq.FeeType = req.FeeType
	}
	withdrawReq.Metainfo = req.Metainfo
	if err = s.wallet.Withdraw(&withdrawReq, toColdStorage); err != nil {
		s.respond(response, nil, err)
		return
	}
	s.respond(response, withdrawReq, nil)
}

func (s *APIServer) withdrawRegular(response http.ResponseWriter, request *http.Request) {
	s.withdraw(false, response, request)
}

func (s *APIServer) withdrawToColdStorage(response http.ResponseWriter, request *http.Request) {
	s.withdraw(true, response, request)
}

func (s *APIServer) getHotStorageAddress(response http.ResponseWriter, request *http.Request) {
	s.respond(response, s.wallet.GetHotWalletAddress(), nil)
}

func (s *APIServer) getTransactions(response http.ResponseWriter, request *http.Request) {
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

func (s *APIServer) getBalance(response http.ResponseWriter, request *http.Request) {
	var respData struct {
		Balance           bitcoin.BitcoinAmount `json:"balance"`
		BalanceWithUnconf bitcoin.BitcoinAmount `json:"balance_including_unconfirmed"`
	}
	var err error
	respData.Balance, respData.BalanceWithUnconf, err = s.wallet.GetBalance()

	s.respond(response, respData, err)
}

func (s *APIServer) getRequiredFromColdStorage(response http.ResponseWriter, request *http.Request) {
	s.respond(response, s.wallet.GetMoneyRequiredFromColdStorage(), nil)
}

func (s *APIServer) cancelPending(response http.ResponseWriter, request *http.Request) {
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

func (s *APIServer) confirmPendingTransaction(response http.ResponseWriter, request *http.Request) {
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

func (s *APIServer) getEvents(response http.ResponseWriter, request *http.Request) {
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

func (s *APIServer) initHTTPAPIServer() {
	m := s.httpServer.Handler.(*http.ServeMux)
	m.HandleFunc("/new_wallet", s.newBitcoinAddress)
	m.HandleFunc("/notify_wallet", s.notifyWalletTxStatusChanged)
	m.HandleFunc("/withdraw", s.withdrawRegular)
	m.HandleFunc("/get_hot_storage_address", s.getHotStorageAddress)
	m.HandleFunc("/get_transactions", s.getTransactions)
	m.HandleFunc("/get_balance", s.getBalance)
	m.HandleFunc("/get_required_from_cold_storage", s.getRequiredFromColdStorage)
	m.HandleFunc("/cancel_pending", s.cancelPending)
	m.HandleFunc("/withdraw_to_cold_storage", s.withdrawToColdStorage)
	m.HandleFunc("/confirm", s.confirmPendingTransaction)
	m.HandleFunc("/get_events", s.getEvents)
}
