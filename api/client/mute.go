package client

import (
	"github.com/onederx/bitcoin-processing/api"
)

func (cli *Client) MuteEventsForTxID(tx string) error {
	return cli.sendHTTPAPIRequest(api.MuteEventsURL, tx, nil)
}
