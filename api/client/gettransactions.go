package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

func (cli *Client) GetTransactions(filter *api.GetTransactionsFilter) ([]*wallet.Transaction, error) {
	var responseData []*wallet.Transaction

	err := cli.sendHTTPAPIRequest(api.GetTransactionsURL, filter, func(response []byte) error {
		return json.Unmarshal(response, &responseData)
	})
	return responseData, err
}
