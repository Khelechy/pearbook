package crdt

import (
	"sync"
	"testing"
)

func TestORSetAdd(t *testing.T) {
	set := NewORSet()

	// Test adding elements
	set.Add("element1", "replica1")
	set.Add("element2", "replica1")

	if !set.Lookup("element1") {
		t.Fatal("Element1 should be in set")
	}

	if !set.Lookup("element2") {
		t.Fatal("Element2 should be in set")
	}

	if set.Lookup("element3") {
		t.Fatal("Element3 should not be in set")
	}
}

func TestORSetRemove(t *testing.T) {
	set := NewORSet()

	set.Add("element1", "replica1")
	set.Add("element2", "replica1")

	if !set.Lookup("element1") {
		t.Fatal("Element1 should be in set before removal")
	}

	set.Remove("element1", "replica1")

	if set.Lookup("element1") {
		t.Fatal("Element1 should be removed from set")
	}

	if !set.Lookup("element2") {
		t.Fatal("Element2 should still be in set")
	}
}

func TestORSetMerge(t *testing.T) {
	set1 := NewORSet()
	set2 := NewORSet()

	set1.Add("element1", "replica1")
	set1.Add("element2", "replica1")

	set2.Add("element2", "replica2")
	set2.Add("element3", "replica2")

	set1.Merge(set2)

	if !set1.Lookup("element1") {
		t.Fatal("Element1 should be in merged set")
	}

	if !set1.Lookup("element2") {
		t.Fatal("Element2 should be in merged set")
	}

	if !set1.Lookup("element3") {
		t.Fatal("Element3 should be in merged set")
	}
}

func TestORSetConcurrent(t *testing.T) {
	set := NewORSet()
	var wg sync.WaitGroup

	// Test concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			set.Add(string(rune('a'+id)), "replica1")
		}(i)
	}

	wg.Wait()

	// Verify all elements were added
	for i := 0; i < 10; i++ {
		element := string(rune('a' + i))
		if !set.Lookup(element) {
			t.Fatalf("Element %s should be in set", element)
		}
	}
}

func TestORSetRemoveNonExistent(t *testing.T) {
	set := NewORSet()

	// Should not panic when removing non-existent element
	set.Remove("nonexistent", "replica1")

	if set.Lookup("nonexistent") {
		t.Fatal("Non-existent element should not be in set")
	}
}

func TestORSetAddRemoveAdd(t *testing.T) {
	set := NewORSet()

	set.Add("element1", "replica1")
	set.Remove("element1", "replica1")
	set.Add("element1", "replica2") // Different replica

	if !set.Lookup("element1") {
		t.Fatal("Element1 should be in set after re-add")
	}
}

// ===== ORMAP TESTS =====

func TestORMapPut(t *testing.T) {
	m := NewORMap()

	m.Put("key1", "value1", "replica1")
	m.Put("key2", "value2", "replica1")

	value1, exists1 := m.Get("key1")
	if !exists1 || value1 != "value1" {
		t.Fatal("Key1 should exist with value1")
	}

	value2, exists2 := m.Get("key2")
	if !exists2 || value2 != "value2" {
		t.Fatal("Key2 should exist with value2")
	}

	_, exists3 := m.Get("key3")
	if exists3 {
		t.Fatal("Key3 should not exist")
	}
}

func TestORMapRemove(t *testing.T) {
	m := NewORMap()

	m.Put("key1", "value1", "replica1")

	value1, exists1 := m.Get("key1")
	if !exists1 || value1 != "value1" {
		t.Fatal("Key1 should exist with value1 before removal")
	}

	m.Remove("key1", "replica1")

	_, exists2 := m.Get("key1")
	if exists2 {
		t.Fatal("Key1 should not exist after removal")
	}
}

func TestORMapUpdate(t *testing.T) {
	m := NewORMap()

	m.Put("key1", "value1", "replica1")
	m.Put("key1", "value2", "replica1") // Update value

	value, exists := m.Get("key1")
	if !exists || value != "value2" {
		t.Fatal("Key1 should have updated value2")
	}
}

func TestORMapMerge(t *testing.T) {
	m1 := NewORMap()
	m2 := NewORMap()

	m1.Put("key1", "value1", "replica1")
	m1.Put("key2", "value2", "replica1")

	m2.Put("key2", "value2_new", "replica2")
	m2.Put("key3", "value3", "replica2")

	m1.Merge(m2)

	value1, exists1 := m1.Get("key1")
	if !exists1 || value1 != "value1" {
		t.Fatal("Key1 should have value1 in merged map")
	}

	value2, exists2 := m1.Get("key2")
	if !exists2 || (value2 != "value2" && value2 != "value2_new") {
		t.Fatalf("Key2 should have either value2 or value2_new in merged map, got: %v", value2)
	}

	value3, exists3 := m1.Get("key3")
	if !exists3 || value3 != "value3" {
		t.Fatal("Key3 should have value3 in merged map")
	}
}

func TestORMapConcurrent(t *testing.T) {
	m := NewORMap()
	var wg sync.WaitGroup

	// Test concurrent puts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id))
			value := string(rune('A' + id))
			m.Put(key, value, "replica1")
		}(i)
	}

	wg.Wait()

	// Verify all entries were added
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		expectedValue := string(rune('A' + i))
		value, exists := m.Get(key)
		if !exists || value != expectedValue {
			t.Fatalf("Key %s should exist with value %s", key, expectedValue)
		}
	}
}

// ===== PNCOUNTER TESTS =====

func TestPNCounterIncrement(t *testing.T) {
	counter := NewPNCounter()

	counter.Increment("replica1", 5)
	if counter.Value() != 5 {
		t.Fatalf("Expected counter value 5, got %d", counter.Value())
	}

	counter.Increment("replica1", 3)
	if counter.Value() != 8 {
		t.Fatalf("Expected counter value 8, got %d", counter.Value())
	}
}

func TestPNCounterDecrement(t *testing.T) {
	counter := NewPNCounter()

	counter.Increment("replica1", 10)
	counter.Decrement("replica1", 3)

	if counter.Value() != 7 {
		t.Fatalf("Expected counter value 7, got %d", counter.Value())
	}

	counter.Decrement("replica1", 10)
	if counter.Value() != -3 {
		t.Fatalf("Expected counter value -3, got %d", counter.Value())
	}
}

func TestPNCounterMerge(t *testing.T) {
	counter1 := NewPNCounter()
	counter2 := NewPNCounter()

	counter1.Increment("replica1", 5)
	counter1.Decrement("replica1", 2)

	counter2.Increment("replica2", 3)
	counter2.Decrement("replica2", 1)

	counter1.Merge(counter2)

	expectedValue := int64((5 - 2) + (3 - 1)) // 3 + 2 = 5
	if counter1.Value() != expectedValue {
		t.Fatalf("Expected merged counter value %d, got %d", expectedValue, counter1.Value())
	}
}

func TestPNCounterConcurrent(t *testing.T) {
	counter := NewPNCounter()
	var wg sync.WaitGroup

	// Test concurrent increments
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Increment("replica1", 1)
		}()
	}

	wg.Wait()

	if counter.Value() != 10 {
		t.Fatalf("Expected counter value 10, got %d", counter.Value())
	}
}

func TestPNCounterZero(t *testing.T) {
	counter := NewPNCounter()

	if counter.Value() != 0 {
		t.Fatalf("Expected initial counter value 0, got %d", counter.Value())
	}

	counter.Increment("replica1", 5)
	counter.Decrement("replica1", 5)

	if counter.Value() != 0 {
		t.Fatalf("Expected counter value 0 after balancing operations, got %d", counter.Value())
	}
}

func TestPNCounterNegative(t *testing.T) {
	counter := NewPNCounter()

	counter.Decrement("replica1", 5)

	if counter.Value() != -5 {
		t.Fatalf("Expected counter value -5, got %d", counter.Value())
	}
}

// ===== CROSS-CRDT TESTS =====

func TestCRDTMergeConsistency(t *testing.T) {
	// Test that merging is commutative
	set1 := NewORSet()
	set2 := NewORSet()

	set1.Add("a", "r1")
	set2.Add("b", "r2")

	// Merge in one order
	set1Copy := NewORSet()
	set1Copy.Add("a", "r1")
	set2Copy := NewORSet()
	set2Copy.Add("b", "r2")

	set1Copy.Merge(set2Copy)
	result1 := set1Copy.Lookup("a") && set1Copy.Lookup("b")

	// Merge in reverse order
	set1.Merge(set2)
	result2 := set1.Lookup("a") && set1.Lookup("b")

	if result1 != result2 {
		t.Fatal("Merge operation should be commutative")
	}
}

func TestCRDTIdempotentMerge(t *testing.T) {
	set := NewORSet()
	set.Add("element", "replica1")

	originalValue := set.Lookup("element")

	// Merge with itself should not change anything
	set.Merge(set)

	if set.Lookup("element") != originalValue {
		t.Fatal("Merge with self should be idempotent")
	}
}

// ===== PERFORMANCE TESTS =====

func BenchmarkORSetAdd(b *testing.B) {
	set := NewORSet()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set.Add(string(rune(i)), "replica1")
	}
}

func BenchmarkORSetContains(b *testing.B) {
	set := NewORSet()
	for i := 0; i < 1000; i++ {
		set.Add(string(rune(i)), "replica1")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set.Lookup(string(rune(i % 1000)))
	}
}

func BenchmarkORMapPut(b *testing.B) {
	m := NewORMap()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := string(rune(i))
		value := string(rune(i + 1000))
		m.Put(key, value, "replica1")
	}
}

func BenchmarkORMapGet(b *testing.B) {
	m := NewORMap()
	for i := 0; i < 1000; i++ {
		key := string(rune(i))
		value := string(rune(i + 1000))
		m.Put(key, value, "replica1")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.Get(string(rune(i % 1000)))
	}
}

func BenchmarkPNCounterIncrement(b *testing.B) {
	counter := NewPNCounter()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.Increment("replica1", 1)
	}
}

func BenchmarkPNCounterValue(b *testing.B) {
	counter := NewPNCounter()
	for i := 0; i < 1000; i++ {
		counter.Increment("replica1", 1)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.Value()
	}
}
