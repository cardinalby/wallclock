package alarm

import "sync"

type broadcaster struct {
	mu sync.RWMutex
	ch chan struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{
		ch: make(chan struct{}),
	}
}

func (b *broadcaster) C() <-chan struct{} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ch
}

func (b *broadcaster) Broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()
	close(b.ch)
	b.ch = make(chan struct{})
}
