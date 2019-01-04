package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

func (cli *Client) Withdraw(request *wallet.WithdrawRequest) (*wallet.WithdrawRequest, error) {
	return cli.withdraw(api.WithdrawURL, request)
}

func (cli *Client) WithdrawToColdStorage(request *wallet.WithdrawRequest) (*wallet.WithdrawRequest, error) {
	return cli.withdraw(api.WithdrawToColdStorageURL, request)
}

func (cli *Client) withdraw(relativeURL string, request *wallet.WithdrawRequest) (*wallet.WithdrawRequest, error) {
	var responseData wallet.WithdrawRequest

	err := cli.sendHTTPAPIRequest(relativeURL, request, func(response []byte) error {
		return json.Unmarshal(response, &responseData)
	})

	return &responseData, err
}
