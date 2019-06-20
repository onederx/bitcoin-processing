package types

type TxNotification struct {
	Transaction
	StatusCode int    `json:"status"`
	StatusStr  string `json:"status_name"`
	IpnType    string `json:"ipn_type"`
	Currency   string `json:"currency"`
	IpnID      string `json:"ipn_id"`
	Seq        int    `json:"seq,omitempty"`
}
