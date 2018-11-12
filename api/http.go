package api

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/shopspring/decimal"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

type httpAPIResponse struct {
	Error  string      `json:"error"`
	Result interface{} `json:"result"`
}

var satoshiInBTCDecimal = decimal.New(1, 8)

func amountFromString(amount string) (uint64, error) {
	amountDecimal, err := decimal.NewFromString(amount)
	if err != nil {
		return 0, err
	}
	return uint64(amountDecimal.Mul(satoshiInBTCDecimal).IntPart()), nil
}

func (s *APIServer) respond(response http.ResponseWriter, data interface{}, err error) {
	var responseBody []byte
	if err != nil {
		response.WriteHeader(500)
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
	var req struct {
		Id      uuid.UUID
		Address string
		Amount  string
		Fee     string
		FeeType string `json:"fee_type"`
	}
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
	withdrawReq.Amount, err = amountFromString(req.Amount)
	if err != nil {
		s.respond(response, nil, err)
		return
	}
	withdrawReq.Fee, err = amountFromString(req.Fee)
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
	var txFilter struct {
		Direction string
		Status    string
	}
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
		Balance           uint64 `json:"balance"`
		BalanceWithUnconf uint64 `json:"balance_including_unconfirmed"`
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

func (s *APIServer) initHTTPAPIServer() {
	m := s.httpServer.Handler.(*http.ServeMux)
	m.HandleFunc("/new-address", s.newBitcoinAddress)
	m.HandleFunc("/notify-wallet", s.notifyWalletTxStatusChanged)
	m.HandleFunc("/withdraw", s.withdrawRegular)
	m.HandleFunc("/get-hot-storage-address", s.getHotStorageAddress)
	m.HandleFunc("/get-transactions", s.getTransactions)
	m.HandleFunc("/get-balance", s.getBalance)
	m.HandleFunc("/get-required-from-cold-storage", s.getRequiredFromColdStorage)
	m.HandleFunc("/cancel-pending", s.cancelPending)
	m.HandleFunc("/withdraw-to-cold-storage", s.withdrawToColdStorage)
}
