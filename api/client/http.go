package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/util"
)

type callbackedHTTPAPIResult struct {
	unmarshalCb func([]byte) error
}

func (g callbackedHTTPAPIResult) UnmarshalJSON(b []byte) error {
	if g.unmarshalCb != nil {
		return g.unmarshalCb(b)
	}
	return nil
}

type callbackedHTTPAPIResponse struct {
	api.GenericHTTPAPIResponse
	Result callbackedHTTPAPIResult `json:"result"`
}

func (cli *Client) sendHTTPAPIRequest(relativeURL string, request interface{}, resultCb func([]byte) error) error {
	var requestBody io.Reader

	if request != nil {
		requestBodyJSON, err := json.Marshal(request)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(requestBodyJSON)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	fullURL, err := util.URLJoin(cli.apiBaseURL, relativeURL)

	if err != nil {
		return err
	}

	resp, err := http.Post(fullURL, "application/json", requestBody)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var apiResponse callbackedHTTPAPIResponse

	apiResponse.Result.unmarshalCb = resultCb

	err = json.NewDecoder(resp.Body).Decode(&apiResponse)

	if err != nil {
		return err
	}

	if apiResponse.Error != "ok" {
		return apiResponse.Error
	}
	return nil
}
