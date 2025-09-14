package dht

import (
	"context"
	"fmt"
	"sync"
)

// SimulatedDHT represents a simulated DHT
type SimulatedDHT struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewSimulatedDHT creates a new simulated DHT
func NewSimulatedDHT() *SimulatedDHT {
	return &SimulatedDHT{
		data: make(map[string]string),
	}
}

// Put stores data in the DHT
func (d *SimulatedDHT) Put(key, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data[key] = value
}

// Get retrieves data from the DHT
func (d *SimulatedDHT) Get(key string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	val, ok := d.data[key]
	return val, ok
}

// PutValue stores data in the DHT
func (d *SimulatedDHT) PutValue(ctx context.Context, key string, value []byte) error {
	d.Put(key, string(value))
	return nil
}

// GetValue retrieves data from the DHT
func (d *SimulatedDHT) GetValue(ctx context.Context, key string) ([]byte, error) {
	val, ok := d.Get(key)
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return []byte(val), nil
}
