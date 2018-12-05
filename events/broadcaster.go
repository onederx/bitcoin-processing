package events

import (
	"log"
	"sync"
)

type broadcastedEvent = *NotificationWithSeq

type broadcaster struct {
	m    sync.Mutex
	subs map[<-chan broadcastedEvent]chan broadcastedEvent
}

type broadcasterWithStorage struct {
	broadcaster
	storage EventStorage
	seqM    sync.Mutex
	seqSubs map[<-chan broadcastedEvent]<-chan broadcastedEvent
}

const channelSize = 1000

func newBroadcaster() *broadcaster {
	return &broadcaster{subs: make(map[<-chan broadcastedEvent]chan broadcastedEvent)}
}

func newBroadcasterWithStorage(storage EventStorage) *broadcasterWithStorage {
	return &broadcasterWithStorage{
		broadcaster: *newBroadcaster(),
		storage:     storage,
		seqSubs:     make(map[<-chan broadcastedEvent]<-chan broadcastedEvent),
	}
}

func (b *broadcaster) Subscribe() <-chan broadcastedEvent {
	b.m.Lock()
	defer b.m.Unlock()

	ch := make(chan broadcastedEvent, channelSize)
	b.subs[ch] = ch

	return ch
}

func (b *broadcaster) Unsubscribe(subch <-chan broadcastedEvent) {
	b.m.Lock()
	defer b.m.Unlock()

	ch, ok := b.subs[subch]
	if !ok {
		return
	}

	close(ch)
	delete(b.subs, subch)
}

func (b *broadcaster) Broadcast(e broadcastedEvent) {
	b.m.Lock()
	defer b.m.Unlock()

	for k, ch := range b.subs {
		select {
		case ch <- e:

		default:
			// too bad, consumer should have been faster
			close(ch)
			delete(b.subs, k)
		}
	}
}

func (b *broadcaster) Close() {
	b.m.Lock()
	defer b.m.Unlock()

	for k, ch := range b.subs {
		close(ch)
		delete(b.subs, k)
	}
}

func (b *broadcasterWithStorage) CheckIfSubscribed(subch <-chan broadcastedEvent) bool {
	b.seqM.Lock()
	defer b.seqM.Unlock()

	_, ok := b.seqSubs[subch]
	return ok
}

func (b *broadcasterWithStorage) sendOldAndPipeNewEventsToClient(resultEventChannel chan broadcastedEvent,
	readEventChannel <-chan broadcastedEvent, storedEvents, eventBuffer []broadcastedEvent) {
	defer close(resultEventChannel)

	lastEventSeq := 0
	for _, storedEvent := range storedEvents {
		lastEventSeq = storedEvent.Seq
		select {
		case resultEventChannel <- storedEvent:

		default:
			// maybe client has unsubscribed already
			// then this is a no-op
			b.unsubscribeFromSeq(resultEventChannel)
			return
		}

	}

	for _, newEvent := range eventBuffer {
		if newEvent.Seq <= lastEventSeq {
			// skip events that were already in DB
			continue
		}
		select {
		case resultEventChannel <- newEvent:

		default:
			// maybe client has unsubscribed already
			// then this is a no-op
			b.unsubscribeFromSeq(resultEventChannel)
			return
		}
	}

	for event := range readEventChannel {
		select {
		case resultEventChannel <- event:

		default:
			b.unsubscribeFromSeq(resultEventChannel)
			return
		}
	}
}

func (b *broadcasterWithStorage) SubscribeFromSeq(seq int) <-chan broadcastedEvent {
	var storedEvents []broadcastedEvent

	b.seqM.Lock()
	defer b.seqM.Unlock()

	readEventChannel := b.Subscribe()
	resultEventChannel := make(chan broadcastedEvent, channelSize)
	b.seqSubs[resultEventChannel] = readEventChannel

	eventBuffer := make([]broadcastedEvent, 0, 100)
	eventsFromStorage := make(chan []broadcastedEvent)

	go func() {
		events, err := b.storage.GetEventsFromSeq(seq)
		if err != nil {
			log.Printf("Error: failed to get events from storage: %s", err)
			eventsFromStorage <- make([]broadcastedEvent, 0)
		} else {
			eventsFromStorage <- events
		}
	}()

waitingForStorage:
	for {
		select {
		case newEvent := <-readEventChannel:
			eventBuffer = append(eventBuffer, newEvent)
		case storedEvents = <-eventsFromStorage:
			break waitingForStorage
		}
	}

	go b.sendOldAndPipeNewEventsToClient(
		resultEventChannel,
		readEventChannel,
		storedEvents,
		eventBuffer,
	)

	return resultEventChannel
}

func (b *broadcasterWithStorage) unsubscribeFromSeq(subch <-chan broadcastedEvent) {
	b.seqM.Lock()
	defer b.seqM.Unlock()

	readEventChannel, ok := b.seqSubs[subch]
	if !ok {
		return
	}

	b.Unsubscribe(readEventChannel)
	delete(b.seqSubs, subch)
}

func (b *broadcasterWithStorage) Unsubscribe(subch <-chan broadcastedEvent) {
	b.m.Lock()

	ch, ok := b.subs[subch]
	if !ok {
		b.m.Unlock()
		b.unsubscribeFromSeq(subch)
		return
	}
	defer b.m.Unlock()

	close(ch)
	delete(b.subs, subch)
}
