package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/dht"
	"github.com/khelechy/pearbook/models"
	"github.com/khelechy/pearbook/node"
)

// MockKDHT simulates KDHT for testing
type MockKDHT struct {
	data map[string][]byte
}

func NewMockKDHT() *MockKDHT {
	return &MockKDHT{data: make(map[string][]byte)}
}

func (m *MockKDHT) PutValue(ctx context.Context, key string, value []byte) error {
	m.data[key] = value
	return nil
}

func (m *MockKDHT) GetValue(ctx context.Context, key string) ([]byte, error) {
	val, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return val, nil
}

func TestCreateGroup(t *testing.T) {
	node := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create signed operation for group creation
	signedOp := models.SignedOperation{
		Operation: "create_group",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"group": map[string]interface{}{
				"id":   "testgroup",
				"name": "Test Group",
			},
			"user": map[string]interface{}{
				"user_name":  "Alice Smith",
				"public_key": "mock-public-key-alice",
			},
		},
		Signature: []byte("mock-signature"), // Mock signature for testing
	}

	err := node.CreateGroup(context.Background(), signedOp)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	if node.GetGroups()["testgroup"] == nil {
		t.Fatal("Group not created in local cache")
	}
}

func TestJoinGroup(t *testing.T) {
	node := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create group first
	createOp := models.SignedOperation{
		Operation: "create_group",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"group": map[string]interface{}{
				"id":   "testgroup",
				"name": "Test Group",
			},
			"user": map[string]interface{}{
				"user_name":  "Alice Smith",
				"public_key": "mock-public-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	node.CreateGroup(context.Background(), createOp)

	// Join group
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Bob Johnson",
				"public_key": "mock-public-key-bob",
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := node.JoinGroup(context.Background(), joinOp)
	if err != nil {
		t.Fatalf("Failed to join group: %v", err)
	}

	// Check that join request was created
	requestID := "testgroup:bob"
	_, exists := node.GetGroups()["testgroup"].PendingJoins.Get(requestID)
	if !exists {
		t.Fatal("Join request not created")
	}

	// Only alice should be a member initially
	members := node.GetGroups()["testgroup"].Members.Keys()
	if len(members) != 1 || members[0] != "alice" {
		t.Fatalf("Expected only alice as member, got %v", members)
	}
}

func TestAddExpense(t *testing.T) {
	node := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create group
	createOp := models.SignedOperation{
		Operation: "create_group",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"group": map[string]interface{}{
				"id":   "testgroup",
				"name": "Test Group",
			},
			"user": map[string]interface{}{
				"user_name":  "Alice Smith",
				"public_key": "mock-public-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	node.CreateGroup(context.Background(), createOp)

	// Bob joins
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Bob Johnson",
				"public_key": "mock-public-key-bob",
			},
		},
		Signature: []byte("mock-signature"),
	}
	node.JoinGroup(context.Background(), joinOp)

	// Approve Bob's join request
	approveOp := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": "testgroup:bob",
		},
		Signature: []byte("mock-signature"),
	}
	err := node.ApproveJoin(context.Background(), approveOp)
	if err != nil {
		t.Fatalf("Failed to approve join: %v", err)
	}

	// Add expense
	expenseOp := models.SignedOperation{
		Operation: "add_expense",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"expense": map[string]interface{}{
				"id":           "exp1",
				"amount":       100.0,
				"description":  "Dinner",
				"participants": []interface{}{"alice", "bob"}, // Use interface{} slice
			},
		},
		Signature: []byte("mock-signature"),
	}
	err = node.AddExpense(context.Background(), expenseOp)
	if err != nil {
		t.Fatalf("Failed to add expense: %v", err)
	}

	exp, ok := node.GetGroups()["testgroup"].Expenses.Get("exp1")
	if !ok || exp == nil {
		t.Fatal("Expense not added to ORMap")
	}

	// Get balances
	balanceOp := models.SignedOperation{
		Operation: "get_balances",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{},
		Signature: []byte("mock-signature"),
	}
	balances, err := node.GetBalances(context.Background(), balanceOp)
	if err != nil {
		t.Fatalf("Failed to get balances: %v", err)
	}
	aliceBalance, exists := balances["alice"]
	if !exists {
		t.Fatal("Alice balance not found")
	}
	amount, ok := aliceBalance["amount"].(float64)
	if !ok {
		t.Fatal("Amount field not found or invalid")
	}
	if amount != 50.0 {
		t.Fatalf("Expected balance 50, got %f", amount)
	}
}

func TestSyncGroup(t *testing.T) {
	node := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create group first
	createOp := models.SignedOperation{
		Operation: "create_group",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"group": map[string]interface{}{
				"id":   "testgroup",
				"name": "Test Group",
			},
			"user": map[string]interface{}{
				"user_name":  "Alice Smith",
				"public_key": "mock-public-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	node.CreateGroup(context.Background(), createOp)

	err := node.SyncGroup(context.Background(), "testgroup")
	if err != nil {
		t.Fatalf("Failed to sync group: %v", err)
	}
	if node.GetGroups()["testgroup"] == nil {
		t.Fatal("Group not synced to local cache")
	}
}

func TestORSetMerge(t *testing.T) {
	set1 := crdt.NewORSet()
	set1.Add("alice", "tag1")
	set2 := crdt.NewORSet()
	set2.Add("bob", "tag2")
	set1.Merge(set2)
	elements := set1.Elements()
	if len(elements) != 2 || !contains(elements, "alice") || !contains(elements, "bob") {
		t.Fatalf("Merge failed, elements: %v", elements)
	}
}

func TestORMapMerge(t *testing.T) {
	m := crdt.NewORMap()
	m.Put("exp1", models.Expense{ID: "exp1"}, "tag1")
	m2 := crdt.NewORMap()
	m2.Put("exp2", models.Expense{ID: "exp2"}, "tag2")
	m.Merge(m2)
	keys := m.Keys()
	if len(keys) != 2 || !contains(keys, "exp1") || !contains(keys, "exp2") {
		t.Fatalf("Merge failed, keys: %v", keys)
	}
}

func TestPNCounterMerge(t *testing.T) {
	c1 := crdt.NewPNCounter()
	c1.Increment("node1", 10)
	c2 := crdt.NewPNCounter()
	c2.Increment("node2", 5)
	c1.Merge(c2)
	if c1.Value() != 15 {
		t.Fatalf("Merge failed, value: %d", c1.Value())
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
