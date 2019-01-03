package client

import (
	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/api"
)

func (cli *Client) Confirm(id uuid.UUID) error {
	return cli.sendHTTPAPIRequest(api.ConfirmURL, id, nil)
}
