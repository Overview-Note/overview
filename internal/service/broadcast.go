package service

import (
	"sync"
	"time"
)

// ChangeEvent announces that the vault changed. LatestSeq is the head of the
// note change sequence and LatestTs is the newest tombstone timestamp, so a
// subscriber can tell whether a note or a deletion changed even when the
// sequence itself did not advance.
type ChangeEvent struct {
	LatestSeq int64     `json:"latestSeq"`
	LatestTs  time.Time `json:"latestTs"`
}

// ChangeBus is an in-process fan-out for vault change notifications. It is
// deliberately non-blocking: a slow subscriber misses intermediate events but
// always observes the latest cursor when it next drains.
type ChangeBus struct {
	mu     sync.Mutex
	subs   map[int]chan ChangeEvent
	nextID int
}

// NewChangeBus creates an empty bus.
func NewChangeBus() *ChangeBus {
	return &ChangeBus{subs: map[int]chan ChangeEvent{}}
}

// Subscribe registers a subscriber and returns its id and a buffered channel.
// Call Unsubscribe with the id when done.
func (b *ChangeBus) Subscribe() (int, <-chan ChangeEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan ChangeEvent, 1)
	b.subs[id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *ChangeBus) Unsubscribe(id int) {
	b.mu.Lock()
	ch, ok := b.subs[id]
	if ok {
		delete(b.subs, id)
	}
	b.mu.Unlock()
	if ok {
		close(ch)
	}
}

// Publish delivers an event to every subscriber. A subscriber that has not
// drained its previous event keeps the older cursor until it drains, at which
// point it will re-query and catch up, so dropping is safe.
func (b *ChangeBus) Publish(ev ChangeEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}
