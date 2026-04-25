package service

import (
	"context"
	"sync"
)

// Broadcaster fans out events to any number of subscribers.
// Safe for concurrent use.
type Broadcaster[T any] struct {
	mu          sync.Mutex
	subscribers map[chan T]struct{}
}

func NewBroadcaster[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{
		subscribers: make(map[chan T]struct{}),
	}
}

// Subscribe returns a channel that receives events until ctx is cancelled.
func (b *Broadcaster[T]) Subscribe(ctx context.Context) <-chan T {
	ch := make(chan T, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}()

	return ch
}

// Send delivers an event to all current subscribers.
// Slow subscribers that have a full buffer will have the event dropped.
func (b *Broadcaster[T]) Send(event T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
