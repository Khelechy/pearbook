package main

import (
	"testing"

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
