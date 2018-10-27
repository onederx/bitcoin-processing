package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

func newBitcoinAddress(response http.ResponseWriter, request *http.Request) {
	var account wallet.Account
	var body, responseBody []byte
	var err error

	if body, err = ioutil.ReadAll(request.Body); err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &account); err != nil {
		panic(err)
	}
	account.Address = wallet.GenerateNewAddress(account)
	if responseBody, err = json.Marshal(account); err != nil {
		panic(err)
	}
	response.Write(responseBody)
}

func handle(urlPattern, method string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(urlPattern, func(response http.ResponseWriter, request *http.Request) {
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

func RunHTTPAPIServer(host string, port int) {
	bindAddress := host + ":" + strconv.Itoa(port)
	log.Printf("Starting HTTP API server on %s", bindAddress)

	handle("/new-address", "", newBitcoinAddress)

	log.Fatal(http.ListenAndServe(bindAddress, nil))
}
