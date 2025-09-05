package crdt

import (
	"sync"
)

// ORSet represents an Observed-Remove Set
type ORSet struct {
	AddSet map[string]map[string]bool // element -> tags
	RemSet map[string]map[string]bool // element -> tags
	mu     sync.RWMutex
}

// NewORSet creates a new ORSet
func NewORSet() *ORSet {
	return &ORSet{
		AddSet: make(map[string]map[string]bool),
		RemSet: make(map[string]map[string]bool),
	}
}

// Add adds an element with a unique tag
func (s *ORSet) Add(element, tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.AddSet[element] == nil {
		s.AddSet[element] = make(map[string]bool)
	}
	s.AddSet[element][tag] = true
}

// Remove removes an element with a tag
func (s *ORSet) Remove(element, tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RemSet[element] == nil {
		s.RemSet[element] = make(map[string]bool)
	}
	s.RemSet[element][tag] = true
}

// Lookup checks if an element is in the set
func (s *ORSet) Lookup(element string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	addTags := s.AddSet[element]
	remTags := s.RemSet[element]
	if addTags == nil {
		return false
	}
	for tag := range addTags {
		if !remTags[tag] {
			return true
		}
	}
	return false
}

// Elements returns the current elements
func (s *ORSet) Elements() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var elems []string
	for elem, addTags := range s.AddSet {
		remTags := s.RemSet[elem]
		for tag := range addTags {
			if !remTags[tag] {
				elems = append(elems, elem)
				break
			}
		}
	}
	return elems
}

// Merge merges another ORSet
func (s *ORSet) Merge(other *ORSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for elem, tags := range other.AddSet {
		if s.AddSet[elem] == nil {
			s.AddSet[elem] = make(map[string]bool)
		}
		for tag := range tags {
			s.AddSet[elem][tag] = true
		}
	}
	for elem, tags := range other.RemSet {
		if s.RemSet[elem] == nil {
			s.RemSet[elem] = make(map[string]bool)
		}
		for tag := range tags {
			s.RemSet[elem][tag] = true
		}
	}
}
