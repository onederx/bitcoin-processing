package events

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// EventBroker is responsible for processing events - sending them to client
// via http callback and websocket and storing them in DB
type EventBroker interface {
	Notify(eventType EventType, data interface{}) error
	SubscribeFromSeq(seq int) chan []*NotificationWithSeq
	UnsubscribeFromSeq(chan []*NotificationWithSeq)
	GetEventsFromSeq(seq int) ([]*NotificationWithSeq, error)
	SendNotifications()

	WithTransaction(sqlTx *sql.Tx) EventBroker

	Run()
}

// Notification is a structure describing an event. It holds Type field telling
// what kind of event it is and Data which is an arbitrary data attached to
// this event.
type Notification struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// NotificationWithSeq is same as Notification, but also has a sequence number.
// NotificationWithSeq is produced when event is stored in DB. Sequence number
// can be used to uniquely identify events and determine their order. Clients
// can use them to tell if they have already seen a particular event or, on the
// contrary, have missed some events - and to request only needed portion of
// events.
type NotificationWithSeq struct {
	Notification
	Seq int `json:"seq"`
}

// EventType is a enum describing type of event.
type EventType int

const (
	// NewAddressEvent is emitted when new address is generated. HTTP callback
	// is not called for this event because new addresses are generated only
	// when requested by HTTP API, so client should already know new address
	// is generated
	NewAddressEvent EventType = iota

	// NewIncomingTxEvent is emitted when new incoming tx is found
	NewIncomingTxEvent

	// IncomingTxConfirmedEvent is emitted when incoming tx gains more
	// confirmations than before
	IncomingTxConfirmedEvent

	// NewOutgoingTxEvent is emitted when new outgoing tx is created and
	// broadcasted to Bitcoin network
	NewOutgoingTxEvent

	// OutgoingTxConfirmedEvent is emitted when outgoing tx gains more
	// confirmations than before
	OutgoingTxConfirmedEvent

	// PendingStatusUpdatedEvent is emitted when status of tx changes and new
	// status is pending (for example status changes from 'pending' to
	// 'pending-cold-storage' or vice versa)
	PendingStatusUpdatedEvent

	// PendingTxCancelledEvent is emitted when pending tx is cancelled
	PendingTxCancelledEvent

	// InvalidEvent is for convertion from other types when value of source type
	// is invalid
	InvalidEvent
)

var eventTypeToStringMap = map[EventType]string{
	NewAddressEvent:           "new-address",
	NewIncomingTxEvent:        "new-incoming-tx",
	IncomingTxConfirmedEvent:  "incoming-tx-confirmed",
	NewOutgoingTxEvent:        "new-outgoing-tx",
	OutgoingTxConfirmedEvent:  "outgoing-tx-confirmed",
	PendingStatusUpdatedEvent: "tx-pending-status-updated",
	PendingTxCancelledEvent:   "pending-tx-cancelled",
}

var stringToEventTypeMap = make(map[string]EventType)

var notificationUnmarshalers = make(map[EventType]func([]byte) (interface{}, error))

func init() {
	for eventType, eventTypeStr := range eventTypeToStringMap {
		stringToEventTypeMap[eventTypeStr] = eventType
	}
}

func (et EventType) String() string {
	eventTypeStr, ok := eventTypeToStringMap[et]
	if !ok {
		return "invalid"
	}
	return eventTypeStr
}

// EventTypeFromString converts string representation of event type to EventType
func EventTypeFromString(eventTypeStr string) (EventType, error) {
	et, ok := stringToEventTypeMap[eventTypeStr]
	if !ok {
		return InvalidEvent, errors.New(
			"Failed to convert string '" + eventTypeStr + "' to event type",
		)
	}
	return et, nil
}

// MarshalJSON serializes EventType to JSON. Resulting JSON value is simply
// a string representation of event type
func (et EventType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + et.String() + "\""), nil
}

// UnmarshalJSON deserializes EventType from JSON. Resulting value is mapped
// from string representation of event type
func (et *EventType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*et, err = EventTypeFromString(j)
	return err
}

func RegisterNotificationUnmarshaler(et EventType, unmarshaler func([]byte) (interface{}, error)) {
	notificationUnmarshalers[et] = unmarshaler
}

type genericNodificationData struct {
	eventType  EventType
	resultData interface{}
}

func (n *genericNodificationData) UnmarshalJSON(b []byte) error {
	unmarshaler, ok := notificationUnmarshalers[n.eventType]

	if !ok {
		var genericData interface{}
		err := json.Unmarshal(b, &genericData)
		if err != nil {
			return err
		}
		n.resultData = genericData
		return nil
	}
	data, err := unmarshaler(b)
	if err != nil {
		return err
	}
	n.resultData = data
	return nil
}

func (n *NotificationWithSeq) UnmarshalJSON(b []byte) error {
	var notificationWithoutData struct {
		Type EventType `json:"type"`
		Seq  int       `json:"seq"`
	}
	err := json.Unmarshal(b, &notificationWithoutData)

	if err != nil {
		return err
	}

	n.Type = notificationWithoutData.Type
	n.Seq = notificationWithoutData.Seq

	var notificationWithGenericData struct {
		Data genericNodificationData `json:"data"`
	}
	notificationWithGenericData.Data.eventType = n.Type
	err = json.Unmarshal(b, &notificationWithGenericData)
	if err != nil {
		return err
	}
	n.Data = notificationWithGenericData.Data.resultData
	return nil
}
