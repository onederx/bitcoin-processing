package events

import (
	"log"
	"sync"
	"time"
)

type broadcastedEvent = *NotificationWithSeq
type broadcastedEventSequence = []broadcastedEvent

type broadcaster struct {
	m    sync.Mutex
	subs map[<-chan broadcastedEvent]chan broadcastedEvent
}

type broadcasterWithStorage struct {
	broadcaster
	storage EventStorage
	seqM    sync.Mutex
	seqSubs map[<-chan broadcastedEventSequence]<-chan broadcastedEvent
}

const channelSize = 10000
const sendEventTimeout = time.Second

func newBroadcaster() *broadcaster {
	return &broadcaster{subs: make(map[<-chan broadcastedEvent]chan broadcastedEvent)}
}

func newBroadcasterWithStorage(storage EventStorage) *broadcasterWithStorage {
	return &broadcasterWithStorage{
		broadcaster: *newBroadcaster(),
		storage:     storage,
		seqSubs:     make(map[<-chan broadcastedEventSequence]<-chan broadcastedEvent),
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

func (b *broadcasterWithStorage) sendOldAndPipeNewEventsToClient(resultEventChannel chan broadcastedEventSequence,
	readEventChannel <-chan broadcastedEvent, storedEvents, eventBuffer []broadcastedEvent) {
	defer close(resultEventChannel)

	if len(storedEvents) > 0 {
		lastEventSeq := storedEvents[len(storedEvents)-1].Seq

		resultEventChannel <- storedEvents

		for i, newEvent := range eventBuffer {
			if newEvent.Seq <= lastEventSeq {
				// skip events that were already in DB
				continue
			} else {
				resultEventChannel <- eventBuffer[i:]
				break
			}
		}
	} else if len(eventBuffer) > 0 {
		resultEventChannel <- eventBuffer
	}

	for event := range readEventChannel {
		select {
		case resultEventChannel <- broadcastedEventSequence{event}:

		default:
			log.Printf(
				"Event broadcaster: event queue overflowed, probably client" +
					" is too slow. Closing event queue",
			)
			b.UnsubscribeFromSeq(resultEventChannel)
			return
		}
	}
}

func (b *broadcasterWithStorage) SubscribeFromSeq(seq int) <-chan broadcastedEventSequence {
	var storedEvents broadcastedEventSequence

	b.seqM.Lock()
	defer b.seqM.Unlock()

	readEventChannel := b.Subscribe()
	resultEventChannel := make(chan broadcastedEventSequence, channelSize)
	b.seqSubs[resultEventChannel] = readEventChannel

	eventBuffer := make(broadcastedEventSequence, 0, 100)
	eventsFromStorage := make(chan broadcastedEventSequence)

	go func() {
		events, err := b.storage.GetEventsFromSeq(seq)
		if err != nil {
			log.Printf("Error: failed to get events from storage: %s", err)
			eventsFromStorage <- make(broadcastedEventSequence, 0)
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

func (b *broadcasterWithStorage) UnsubscribeFromSeq(subch <-chan broadcastedEventSequence) {
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
	defer b.m.Unlock()

	ch, ok := b.subs[subch]
	if !ok {
		return
	}

	close(ch)
	delete(b.subs, subch)
}
