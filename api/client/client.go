package client

type Client struct {
	apiBaseURL       string
	websocketClients []*WebsocketClient
}

func NewClient(apiBaseURL string) *Client {
	return &Client{
		apiBaseURL: apiBaseURL,
	}
}
