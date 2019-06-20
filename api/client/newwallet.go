package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/wallet"
)

func (cli *Client) NewWallet(metainfo interface{}) (*wallet.Account, error) {
	var responseData wallet.Account

	err := cli.sendHTTPAPIRequest(api.NewWalletURL, metainfo, func(response []byte) error {
		return json.Unmarshal(response, &responseData)
	})
	return &responseData, err
}
