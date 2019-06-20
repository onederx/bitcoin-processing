package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/wallet/types"
)

func (cli *Client) GetTransactions(filter *api.GetTransactionsFilter) ([]*types.Transaction, error) {
	var responseData []*types.Transaction

	err := cli.sendHTTPAPIRequest(api.GetTransactionsURL, filter, func(response []byte) error {
		return json.Unmarshal(response, &responseData)
	})
	return responseData, err
}
