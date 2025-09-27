package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khelechy/pearbook/pearbook_core/crdt"
	"github.com/khelechy/pearbook/pearbook_core/dht"
	"github.com/khelechy/pearbook/pearbook_core/models"
	"github.com/khelechy/pearbook/pearbook_core/node"
	"github.com/khelechy/pearbook/pearbook_core/server"
)

// ===== NODE TESTS =====

func TestNodeCreation(t *testing.T) {
	n := node.NewNode()
	if n == nil {
		t.Fatal("Failed to create node")
	}
	if n.ID == "" {
		t.Fatal("Node ID not set")
	}
	if n.KDHT != nil {
		t.Fatal("Expected KDHT to be nil for basic node")
	}
}

func TestNodeWithKDHT(t *testing.T) {
	kdht := dht.NewSimulatedDHT()
	n := node.NewNodeWithKDHT(kdht)
	if n == nil {
		t.Fatal("Failed to create node with KDHT")
	}
	if n.KDHT != kdht {
		t.Fatal("KDHT not properly set")
	}
}

// ===== GROUP MANAGEMENT TESTS =====

func TestCreateGroupValidation(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Test invalid operation type
	signedOp := models.SignedOperation{
		Operation: "invalid_op",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{},
		Signature: []byte("mock-signature"),
	}
	err := n.CreateGroup(context.Background(), signedOp)
	if err == nil {
		t.Fatal("Expected error for invalid operation type")
	}

	// Test missing group data
	signedOp.Operation = "create_group"
	err = n.CreateGroup(context.Background(), signedOp)
	if err == nil {
		t.Fatal("Expected error for missing group data")
	}
}

func TestJoinGroupValidation(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Test joining non-existent group
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "nonexistent",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Bob",
				"public_key": "mock-key",
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := n.JoinGroup(context.Background(), joinOp)
	if err == nil {
		t.Fatal("Expected error when joining non-existent group")
	}
}

func TestDuplicateJoinRequest(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

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
				"user_name":  "Alice",
				"public_key": "mock-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.CreateGroup(context.Background(), createOp)

	// First join request
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Bob",
				"public_key": "mock-key-bob",
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := n.JoinGroup(context.Background(), joinOp)
	if err != nil {
		t.Fatalf("First join request failed: %v", err)
	}

	// Duplicate join request should fail
	err = n.JoinGroup(context.Background(), joinOp)
	if err == nil {
		t.Fatal("Expected error for duplicate join request")
	}
}

// ===== EXPENSE MANAGEMENT TESTS =====

func TestAddExpenseValidation(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create and setup group
	setupTestGroup(t, n, "testgroup", "alice", "bob")

	// Test adding expense with non-member participant
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
				"participants": []interface{}{"alice", "charlie"}, // charlie is not a member
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := n.AddExpense(context.Background(), expenseOp)
	if err == nil {
		t.Fatal("Expected error when adding expense with non-member participant")
	}
}

func TestExpenseSplitCalculation(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())
	setupTestGroup(t, n, "testgroup", "alice", "bob")

	// Add expense with 3 participants
	expenseOp := models.SignedOperation{
		Operation: "add_expense",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"expense": map[string]interface{}{
				"id":           "exp1",
				"amount":       90.0,
				"description":  "Dinner",
				"participants": []interface{}{"alice", "bob"},
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.AddExpense(context.Background(), expenseOp)

	// Check balances
	balanceOp := models.SignedOperation{
		Operation: "get_balances",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{},
		Signature: []byte("mock-signature"),
	}
	balances, err := n.GetBalances(context.Background(), balanceOp)
	if err != nil {
		t.Fatalf("Failed to get balances: %v", err)
	}

	aliceBalance, exists := balances["alice"]
	if !exists {
		t.Fatal("Alice balance not found")
	}
	amount := aliceBalance["amount"].(float64)
	if amount != 45.0 { // 90 / 2
		t.Fatalf("Expected balance 45.0, got %f", amount)
	}
}

// ===== APPROVAL SYSTEM TESTS =====

func TestApproveJoinValidation(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())
	setupTestGroup(t, n, "testgroup", "alice", "bob")

	// Try to approve non-existent request
	approveOp := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": "nonexistent",
		},
		Signature: []byte("mock-signature"),
	}
	err := n.ApproveJoin(context.Background(), approveOp)
	if err == nil {
		t.Fatal("Expected error when approving non-existent request")
	}

	// Try to approve with non-member
	approveOp.Data["request_id"] = "testgroup:bob"
	approveOp.UserID = "charlie" // not a member
	err = n.ApproveJoin(context.Background(), approveOp)
	if err == nil {
		t.Fatal("Expected error when non-member tries to approve")
	}
}

func TestSingleApproval(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

	// Create group with multiple members
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
				"user_name":  "Alice",
				"public_key": "mock-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.CreateGroup(context.Background(), createOp)

	// Add bob and charlie as members
	addMember(t, n, "testgroup", "bob")
	addMember(t, n, "testgroup", "charlie")

	// Diana joins
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "testgroup",
		UserID:    "diana",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Diana",
				"public_key": "mock-key-diana",
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.JoinGroup(context.Background(), joinOp)

	// Alice approves (single approval should be sufficient)
	approveOp := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": "testgroup:diana",
		},
		Signature: []byte("mock-signature"),
	}
	n.ApproveJoin(context.Background(), approveOp)

	// Check that Diana is now a member
	members := n.GetGroups()["testgroup"].Members.Keys()
	if !contains(members, "diana") {
		t.Fatalf("Diana should be a member after single approval, members: %v", members)
	}
}

func TestDuplicateApproval(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())

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
				"user_name":  "Alice",
				"public_key": "mock-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.CreateGroup(context.Background(), createOp)

	// Bob joins
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   "testgroup",
		UserID:    "bob",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  "Bob",
				"public_key": "mock-key-bob",
			},
		},
		Signature: []byte("mock-signature"),
	}
	n.JoinGroup(context.Background(), joinOp)

	// Alice approves first time (should succeed)
	approveOp1 := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": "testgroup:bob",
		},
		Signature: []byte("mock-signature"),
	}
	err := n.ApproveJoin(context.Background(), approveOp1)
	if err != nil {
		t.Fatalf("First approval should succeed: %v", err)
	}

	// Alice tries to approve again (should fail)
	approveOp2 := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   "testgroup",
		UserID:    "alice",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": "testgroup:bob",
		},
		Signature: []byte("mock-signature"),
	}
	err = n.ApproveJoin(context.Background(), approveOp2)
	if err == nil {
		t.Fatal("Second approval should fail")
	}
	if !strings.Contains(err.Error(), "user is already a member of the group") {
		t.Fatalf("Expected 'user is already a member of the group' error, got: %v", err)
	}
}

// ===== CRDT TESTS =====

func TestORSetConcurrency(t *testing.T) {
	set := crdt.NewORSet()

	// Test concurrent adds
	done := make(chan bool, 2)
	go func() {
		for i := 0; i < 100; i++ {
			set.Add(fmt.Sprintf("item%d", i), "tag1")
		}
		done <- true
	}()
	go func() {
		for i := 100; i < 200; i++ {
			set.Add(fmt.Sprintf("item%d", i), "tag2")
		}
		done <- true
	}()

	<-done
	<-done

	elements := set.Elements()
	if len(elements) != 200 {
		t.Fatalf("Expected 200 elements, got %d", len(elements))
	}
}

func TestPNCounterPrecision(t *testing.T) {
	counter := crdt.NewPNCounter()

	// Test with int64 values (representing cents for precision)
	counter.Increment("node1", 1050) // $10.50 in cents
	counter.Increment("node2", 525)  // $5.25 in cents

	expected := int64(1575) // (10.50 + 5.25) * 100
	if counter.Value() != expected {
		t.Fatalf("Expected %d, got %d", expected, counter.Value())
	}
}

// ===== SERVER TESTS =====

func TestServerCreateGroup(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())
	srv := &server.Server{Node: n}

	// Create test request
	data := models.SignedOperation{
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
				"user_name":  "Alice",
				"public_key": "mock-key-alice",
			},
		},
		Signature: []byte("mock-signature"),
	}

	body, _ := json.Marshal(data)
	req := httptest.NewRequest("POST", "/createGroup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.HandleCreateGroup(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestServerInvalidMethod(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())
	srv := &server.Server{Node: n}

	req := httptest.NewRequest("GET", "/createGroup", nil)
	w := httptest.NewRecorder()

	srv.HandleCreateGroup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Expected status 405, got %d", w.Code)
	}
}

func TestServerInvalidJSON(t *testing.T) {
	n := node.NewNodeWithKDHT(dht.NewSimulatedDHT())
	srv := &server.Server{Node: n}

	req := httptest.NewRequest("POST", "/createGroup", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	srv.HandleCreateGroup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

// ===== UTILITY FUNCTIONS =====

func setupTestGroup(t *testing.T, n *node.Node, groupID, creatorID, joinerID string) {
	// Create group
	createOp := models.SignedOperation{
		Operation: "create_group",
		GroupID:   groupID,
		UserID:    creatorID,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"group": map[string]interface{}{
				"id":   groupID,
				"name": "Test Group",
			},
			"user": map[string]interface{}{
				"user_name":  creatorID + " Smith",
				"public_key": "mock-key-" + creatorID,
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := n.CreateGroup(context.Background(), createOp)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Add joiner as member
	addMember(t, n, groupID, joinerID)
}

func addMember(t *testing.T, n *node.Node, groupID, userID string) {
	// Join group
	joinOp := models.SignedOperation{
		Operation: "join_group",
		GroupID:   groupID,
		UserID:    userID,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"user_name":  userID + " Johnson",
				"public_key": "mock-key-" + userID,
			},
		},
		Signature: []byte("mock-signature"),
	}
	err := n.JoinGroup(context.Background(), joinOp)
	if err != nil {
		t.Fatalf("Failed to join group: %v", err)
	}

	// Approve join (assuming single member group for simplicity)
	approveOp := models.SignedOperation{
		Operation: "approve_join",
		GroupID:   groupID,
		UserID:    "alice", // creator
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"request_id": groupID + ":" + userID,
		},
		Signature: []byte("mock-signature"),
	}
	err = n.ApproveJoin(context.Background(), approveOp)
	if err != nil {
		t.Fatalf("Failed to approve join: %v", err)
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
