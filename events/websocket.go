package events

import (
	"log"
	"time"
)

func (e *eventBroker) sendWSNotifications() {
	err := e.eventBroadcaster.Broadcast()

	if err != nil {
		log.Printf(
			"Failed to broadcast events via websocket: %v, will retry",
			err,
		)
		time.AfterFunc(3*time.Second, e.triggerWSNotificationSending)
	}
}
