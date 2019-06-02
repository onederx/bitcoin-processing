package events

import (
	"log"
	"sync"

	"github.com/onederx/bitcoin-processing/util"
)

type broadcastedEvent = *NotificationWithSeq
type broadcastedEventSequence = []broadcastedEvent

const channelSize = 10000

type broadcasterWithStorage struct {
	mu sync.Mutex

	storage EventStorage
	subs    map[chan broadcastedEventSequence]int
	minSeq  int
}

func newBroadcasterWithStorage(storage EventStorage) *broadcasterWithStorage {
	return &broadcasterWithStorage{
		storage: storage,
		subs:    make(map[chan broadcastedEventSequence]int),
	}
}

func (b *broadcasterWithStorage) Broadcast() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	events, err := b.storage.GetEventsFromSeq(b.minSeq)

	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	nextSeq := events[len(events)-1].Seq + 1

	for ch, seq := range b.subs {
		for i, event := range events {
			if event.Seq >= seq {
				// we know client wants events starting with this one
				select {
				case ch <- events[i:]:
					b.subs[ch] = nextSeq
				default:
					log.Printf(
						"ws event broadcaster: disconnecting client due to " +
							"oveflow of his channel")
					close(ch)
					delete(b.subs, ch)
				}
				break
			}
		}
	}

	b.minSeq = nextSeq

	return nil
}

func (b *broadcasterWithStorage) SubscribeFromSeq(seq int) chan broadcastedEventSequence {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan broadcastedEventSequence, channelSize)
	b.subs[ch] = seq
	b.minSeq = util.Min(seq, b.minSeq)

	return ch
}

func (b *broadcasterWithStorage) Unsubscribe(subch chan broadcastedEventSequence) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, ok := b.subs[subch]
	if !ok {
		return
	}

	close(subch)
	delete(b.subs, subch)
}

func (b *broadcasterWithStorage) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
}
