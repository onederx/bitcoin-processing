package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
)

func (cli *Client) GetHotStorageAddress() (string, error) {
	var result string

	err := cli.sendHTTPAPIRequest(api.GetHotStorageAddressURL, nil, func(response []byte) error {
		return json.Unmarshal(response, &result)
	})
	return result, err
}

func (cli *Client) GetBalance() (*api.BalanceInfo, error) {
	var responseData api.BalanceInfo

	err := cli.sendHTTPAPIRequest(api.GetBalanceURL, nil, func(response []byte) error {
		return json.Unmarshal(response, &responseData)
	})
	return &responseData, err
}

func (cli *Client) GetRequiredFromColdStorage() (bitcoin.BTCAmount, error) {
	var result bitcoin.BTCAmount

	err := cli.sendHTTPAPIRequest(api.GetRequiredFromColdStorageURL, nil, func(response []byte) error {
		return json.Unmarshal(response, &result)
	})
	return result, err
}
