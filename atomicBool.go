package apns

import (
	"sync"
)

type aBool struct {
	value bool
	mu    sync.RWMutex
}

func (b *aBool) Is() bool {
	b.mu.RLock()
	var result = b.value
	b.mu.RUnlock()
	return result
}

func (b *aBool) Set(v bool) {
	b.mu.Lock()
	b.value = v
	b.mu.Unlock()
}
