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

// Remove removes a key
func (m *ORMap) Remove(key, tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tags, exists := m.Data[key]; exists {
		if len(tag) > 0 {
			delete(tags, tag) // Remove only the specific tag
			if len(tags) == 0 {
				delete(m.Data, key) // Remove key if no tags remain
			}
		} else {
			delete(m.Data, key) // Remove key if no tag was passed
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

// GetAll returns all values for a key (for merging concurrent updates)
func (m *ORMap) GetAll(key string) []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if tags, ok := m.Data[key]; ok && len(tags) > 0 {
		values := make([]interface{}, 0, len(tags))
		for _, v := range tags {
			values = append(values, v)
		}
		return values
	}
	return nil
}

// Merge merges another ORMap, handling both additions and deletions
func (m *ORMap) Merge(other *ORMap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// First, add/update all entries from other
	for key, tags := range other.Data {
		if m.Data[key] == nil {
			m.Data[key] = make(map[string]interface{})
		}
		for tag, val := range tags {
			m.Data[key][tag] = val
		}
	}
	
	// Then, remove keys that are not present in other (deletions)
	for key := range m.Data {
		if other.Data[key] == nil {
			delete(m.Data, key)
		}
	}
}
