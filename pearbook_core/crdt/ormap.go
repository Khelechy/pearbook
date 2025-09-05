package crdt

import (
	"sync"
)

// ORMap represents an Observed-Remove Map
type ORMap struct {
	Data map[string]map[string]interface{} // key -> tag -> value
	mu   sync.RWMutex
}

// NewORMap creates a new ORMap
func NewORMap() *ORMap {
	return &ORMap{
		Data: make(map[string]map[string]interface{}),
	}
}

// Put adds or updates a key-value pair with a tag
func (m *ORMap) Put(key string, value interface{}, tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Data[key] == nil {
		m.Data[key] = make(map[string]interface{})
	}
	m.Data[key][tag] = value
}

// Remove removes a key with a tag
func (m *ORMap) Remove(key, tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Data[key] != nil {
		delete(m.Data[key], tag)
		if len(m.Data[key]) == 0 {
			delete(m.Data, key)
		}
	}
}

// Get retrieves the value for a key (latest by tag, assuming tags are unique)
func (m *ORMap) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if tags, ok := m.Data[key]; ok && len(tags) > 0 {
		// Return the first value (in a real impl, might need to resolve conflicts)
		for _, v := range tags {
			return v, true
		}
	}
	return nil, false
}

// Keys returns all keys
func (m *ORMap) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.Data))
	for k := range m.Data {
		keys = append(keys, k)
	}
	return keys
}

// Merge merges another ORMap
func (m *ORMap) Merge(other *ORMap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, tags := range other.Data {
		if m.Data[key] == nil {
			m.Data[key] = make(map[string]interface{})
		}
		for tag, val := range tags {
			m.Data[key][tag] = val
		}
	}
}
