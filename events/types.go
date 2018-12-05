package events

import (
	"errors"
)

type EventType int

const (
	NewAddressEvent EventType = iota
	CheckTxStatusEvent
	NewIncomingTxEvent
	IncomingTxConfirmedEvent
	NewOutgoingTxEvent
	OutgoingTxConfirmedEvent
	PendingStatusUpdatedEvent
	PendingTxCancelledEvent
	InvalidEvent
)

var eventTypeToStringMap map[EventType]string = map[EventType]string{
	NewAddressEvent:           "new-address",
	CheckTxStatusEvent:        "check-tx-status",
	NewIncomingTxEvent:        "new-incoming-tx",
	IncomingTxConfirmedEvent:  "incoming-tx-confirmed",
	NewOutgoingTxEvent:        "new-outgoing-tx",
	OutgoingTxConfirmedEvent:  "outgoing-tx-confirmed",
	PendingStatusUpdatedEvent: "tx-pending-status-updated",
	PendingTxCancelledEvent:   "pending-tx-cancelled",
}

var stringToEventTypeMap map[string]EventType = make(map[string]EventType)

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

func EventTypeFromString(eventTypeStr string) (EventType, error) {
	et, ok := stringToEventTypeMap[eventTypeStr]
	if !ok {
		return InvalidEvent, errors.New(
			"Failed to convert string '" + eventTypeStr + "' to event type",
		)
	}
	return et, nil
}

func (e EventType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + e.String() + "\""), nil
}

type Notification struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

type NotificationWithSeq struct {
	Notification
	Seq int `json:"seq"`
}
