package client

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/events"
)

func (cli *Client) GetEvents(startSeq int) ([]*events.NotificationWithSeq, error) {
	var responseData []*events.NotificationWithSeq

	err := cli.sendHTTPAPIRequest(
		api.GetEventsURL,
		&api.SubscribeMessage{Seq: startSeq},
		func(response []byte) error {
			return json.Unmarshal(response, &responseData)
		},
	)
	return responseData, err
}
