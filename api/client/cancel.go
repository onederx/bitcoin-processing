package client

import (
	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/api"
)

func (cli *Client) Cancel(id uuid.UUID) error {
	return cli.sendHTTPAPIRequest(api.CancelPendingURL, id, nil)
}
