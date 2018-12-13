package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type bitcoindRequest struct {
	Method string
}

type bitcoindResponse struct {
	Result string
	Error  *struct {
		Code    int
		Message string
	}
}

type addingMoneyTransport struct{}

const delay = time.Second

var nodeAddr string
var payingNodeAddr string

func addMoneyToWalletAfterDelay(address, authorization string) {
	time.Sleep(delay)
	log.Printf("Will now send money to %s", address)

	var sendToAddressRequest struct {
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	sendToAddressRequest.Method = "sendtoaddress"
	sendToAddressRequest.Params = append(sendToAddressRequest.Params, address)
	sendToAddressRequest.Params = append(sendToAddressRequest.Params, 2)

	payerURL := "http://" + payingNodeAddr
	requestBody, err := json.Marshal(sendToAddressRequest)
	if err != nil {
		log.Printf("Error: could not Marshal struct %v to JSON: %s", sendToAddressRequest, err)
		return
	}
	request, err := http.NewRequest("POST", payerURL, bytes.NewReader(requestBody))
	if err != nil {
		log.Printf("Failed to construct new request: %s", err)
		return
	}
	request.Header.Add("Authorization", authorization)
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("Error sending sendtoaddress request %s", err)
		return
	}
	defer resp.Body.Close()
	var response bitcoindResponse
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading resp body %s", err)
		return
	}
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		log.Printf("Error unmarshaling resp body as JSON %s", err)
		return
	}
	if response.Error == nil {
		log.Print("OK: successfully sent money to address")
	}

}

func (t *addingMoneyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	isGetNewAddress := false
	authorization := ""

	if req.Body != nil {
		bodyBytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))
		var nodeRequest bitcoindRequest
		err = json.Unmarshal(bodyBytes, &nodeRequest)
		if err != nil {
			log.Printf("Failed to unmarshal request body %s as JSON: %s", bodyBytes, err)
			return nil, err
		}
		if nodeRequest.Method == "getnewaddress" {
			isGetNewAddress = true
			log.Print("Caught getnewaddress request, will check response for address value")
			authorization = req.Header.Get("authorization")
			log.Printf("Authorization caught: %s", authorization)
		}

	}
	res, err := http.DefaultTransport.RoundTrip(req)

	if err != nil {
		log.Printf("Error proxying request %v to node: %s", req, err)
		return nil, err
	}

	if !isGetNewAddress {
		return res, nil
	}
	if res.Body == nil {
		log.Printf("Error: getnewaddress response has no body")
		return res, nil
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	res.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))
	var nodeResponse bitcoindResponse
	err = json.Unmarshal(bodyBytes, &nodeResponse)
	if err != nil {
		log.Printf("Failed to unmarshal response body %s as JSON: %s", bodyBytes, err)
		return res, nil
	}
	if nodeResponse.Error != nil {
		log.Printf("Node response has error %v", nodeResponse.Error)
		return res, nil
	}
	address := nodeResponse.Result

	log.Printf("Successfully intercepted new wallet address %s", address)

	go addMoneyToWalletAfterDelay(address, authorization)

	return res, nil
}

func main() {
	var bindAddr string

	flag.StringVar(&bindAddr, "bind", "127.0.0.1:9332", "Listen address")
	flag.StringVar(&nodeAddr, "node", "127.0.0.1:8332", "Address of bitcoind node")
	flag.StringVar(&payingNodeAddr, "payer", "127.0.0.1:38332", "Address of bitcoind node that will transfer money")

	flag.Parse()

	rpURL, err := url.Parse("http://" + nodeAddr)

	if err != nil {
		log.Fatal(err)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(rpURL)

	reverseProxy.Transport = &addingMoneyTransport{}

	err = http.ListenAndServe(bindAddr, reverseProxy)
	if err != nil {
		log.Fatal(err)
	}
}
