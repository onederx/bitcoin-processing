package events

import (
	"testing"
	"time"
)

func TestMoreEventsThanChannelSize(t *testing.T) {
	type DummyEventData struct {
		EventId int
	}
	const msgCount = channelSize*2 + 100

	storage := NewEventStorage(nil)
	bcaster := newBroadcasterWithStorage(storage)

	for i := 0; i < msgCount; i++ {
		storage.StoreEvent(Notification{
			Type: NewAddressEvent,
			Data: DummyEventData{EventId: i},
		})
	}

	sub := bcaster.SubscribeFromSeq(0)
	defer bcaster.UnsubscribeFromSeq(sub)

	done := make(chan struct{})
	events := make(chan *NotificationWithSeq)

	go func() {
		for {
			select {
			case eventSequence := <-sub:
				for _, event := range eventSequence {
					select {
					case events <- event:
					case <-done:
						return
					}
				}
			case <-done:
				return
			}
		}
	}()

	defer close(done)

	for i := 0; i < msgCount; i++ {
		var event *NotificationWithSeq
		select {
		case event = <-events:
		case <-time.After(1 * time.Second):
			t.Fatal(
				"Timed out waiting for next event from broadcaster, most " +
					"probably it got lost")
		}
		if event.Seq != i {
			t.Fatalf("Expected next event sequence number to be %d, got %d",
				i, event.Seq)
		}
		eventData := event.Data.(DummyEventData)
		if eventData.EventId != i {
			t.Fatalf("Error: expected event data for event %d, got for %d",
				i, eventData.EventId)
		}
	}
}
