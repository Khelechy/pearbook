package crdt

import (
	"sync"
)

// PNCounter represents a Positive-Negative Counter
type PNCounter struct {
	P  map[string]int64 // positive increments
	N  map[string]int64 // negative increments
	mu sync.RWMutex
}

// NewPNCounter creates a new PNCounter
func NewPNCounter() *PNCounter {
	return &PNCounter{
		P: make(map[string]int64),
		N: make(map[string]int64),
	}
}

// Increment increments the counter by delta
func (c *PNCounter) Increment(id string, delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.P[id] += delta
}

// Decrement decrements the counter by delta
func (c *PNCounter) Decrement(id string, delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.N[id] += delta
}

// Value returns the current value
func (c *PNCounter) Value() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var p, n int64
	for _, v := range c.P {
		p += v
	}
	for _, v := range c.N {
		n += v
	}
	return p - n
}

// Merge merges another PNCounter
func (c *PNCounter) Merge(other *PNCounter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, v := range other.P {
		c.P[id] = max(c.P[id], v)
	}
	for id, v := range other.N {
		c.N[id] = max(c.N[id], v)
	}
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
