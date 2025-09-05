package main

import (
	"testing"

	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/models"
)

func TestCreateGroup(t *testing.T) {
	node := NewNode()
	err := node.CreateGroup("testgroup", "Test Group", "alice")
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	if node.Groups["testgroup"] == nil {
		t.Fatal("Group not created in local cache")
	}
	val, ok := node.DHT.Get("group:testgroup")
	if !ok || val == "" {
		t.Fatal("Group not stored in DHT")
	}
}

func TestJoinGroup(t *testing.T) {
	node := NewNode()
	node.CreateGroup("testgroup", "Test Group", "alice")
	err := node.JoinGroup("testgroup", "bob")
	if err != nil {
		t.Fatalf("Failed to join group: %v", err)
	}
	group := node.Groups["testgroup"]
	members := group.Members.Elements()
	if len(members) != 2 {
		t.Fatalf("Expected 2 members, got %d", len(members))
	}
	if members[1] != "bob" {
		t.Fatal("Bob not added to members")
	}
}

func TestAddExpense(t *testing.T) {
	node := NewNode()
	node.CreateGroup("testgroup", "Test Group", "alice")
	node.JoinGroup("testgroup", "bob")
	expense := models.Expense{
		ID:           "exp1",
		Amount:       100.0,
		Description:  "Dinner",
		Payer:        "alice",
		Participants: []string{"alice", "bob"},
	}
	err := node.AddExpense("testgroup", expense)
	if err != nil {
		t.Fatalf("Failed to add expense: %v", err)
	}
	group := node.Groups["testgroup"]
	exp, ok := group.Expenses.Get("exp1")
	if !ok || exp == nil {
		t.Fatal("Expense not added to ORMap")
	}
	balances := node.GetBalances("testgroup", "bob")
	if balances["alice"] != 50.0 {
		t.Fatalf("Expected balance 50, got %f", balances["alice"])
	}
}

func TestSyncGroup(t *testing.T) {
	node := NewNode()
	node.CreateGroup("testgroup", "Test Group", "alice")
	err := node.SyncGroup("testgroup")
	if err != nil {
		t.Fatalf("Failed to sync group: %v", err)
	}
	if node.Groups["testgroup"] == nil {
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
