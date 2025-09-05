package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/dht"
	"github.com/khelechy/pearbook/models"
)

// Node represents a peer node
type Node struct {
	DHT    *dht.SimulatedDHT
	Groups map[string]*models.Group // local cache
	mu     sync.RWMutex
}

// NewNode creates a new node
func NewNode() *Node {
	return &Node{
		DHT:    dht.NewSimulatedDHT(),
		Groups: make(map[string]*models.Group),
	}
}

// HTTP Handlers

func (n *Node) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string `json:"groupId"`
		Name    string `json:"name"`
		Creator string `json:"creator"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := n.CreateGroup(req.GroupID, req.Name, req.Creator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Group created")
}

func (n *Node) handleJoinGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string `json:"groupId"`
		UserID  string `json:"userId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := n.JoinGroup(req.GroupID, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Joined group")
}

func (n *Node) handleAddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string         `json:"groupId"`
		Expense models.Expense `json:"expense"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := n.AddExpense(req.GroupID, req.Expense)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Expense added")
}

func (n *Node) handleGetBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	groupID := r.URL.Query().Get("groupId")
	userID := r.URL.Query().Get("userId")
	// Sync before getting balances
	n.SyncGroup(groupID)
	balances := n.GetBalances(groupID, userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balances)
}

// CreateGroup creates a new group
func (n *Node) CreateGroup(groupID, name string, creator string) error {
	group := &models.Group{
		ID:       groupID,
		Name:     name,
		Members:  crdt.NewORSet(),
		Expenses: crdt.NewORMap(),
		Balances: make(map[string]map[string]*crdt.PNCounter),
	}

	group.Members.Add(creator, "tag1") // unique tag

	n.mu.Lock()
	n.Groups[groupID] = group
	n.mu.Unlock()

	data, _ := json.Marshal(group)
	n.DHT.Put("group:"+groupID, string(data))
	return nil
}

// JoinGroup joins an existing group
func (n *Node) JoinGroup(groupID, userID string) error {
	// Fetch group from DHT
	val, ok := n.DHT.Get("group:" + groupID)
	if !ok {
		return fmt.Errorf("group not found")
	}

	var group models.Group
	json.Unmarshal([]byte(val), &group)

	// Add member if not already
	members := group.Members.Elements()
	found := false
	for _, m := range members {
		if m == userID {
			found = true
			break
		}
	}
	if !found {
		group.Members.Add(userID, "tag2") // unique tag
		group.Balances[userID] = make(map[string]*crdt.PNCounter)
	}

	n.mu.Lock()
	n.Groups[groupID] = &group
	n.mu.Unlock()

	data, _ := json.Marshal(group)
	n.DHT.Put("group:"+groupID, string(data))
	return nil
}

// AddExpense adds an expense to a group
func (n *Node) AddExpense(groupID string, expense models.Expense) error {
	n.mu.Lock()
	group, exists := n.Groups[groupID]
	n.mu.Unlock()
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Calculate splits (equal split for simplicity)
	numParticipants := len(expense.Participants)
	splitAmount := expense.Amount / float64(numParticipants)
	expense.Splits = make(map[string]float64)
	for _, p := range expense.Participants {
		if p != expense.Payer {
			expense.Splits[p] = splitAmount
		}
	}

	group.Expenses.Put(expense.ID, expense, "tag-exp")

	// Update balances
	for user, owed := range expense.Splits {
		if group.Balances[user] == nil {
			group.Balances[user] = make(map[string]*crdt.PNCounter)
		}
		if group.Balances[user][expense.Payer] == nil {
			group.Balances[user][expense.Payer] = crdt.NewPNCounter()
		}
		group.Balances[user][expense.Payer].Increment("node", int64(owed*100))
	}

	data, _ := json.Marshal(group)
	n.DHT.Put("group:"+groupID, string(data))
	return nil
}

// SyncGroup syncs group data from DHT
func (n *Node) SyncGroup(groupID string) error {
	val, ok := n.DHT.Get("group:" + groupID)
	if !ok {
		return fmt.Errorf("group not found")
	}

	var group models.Group
	json.Unmarshal([]byte(val), &group)

	n.mu.Lock()
	n.Groups[groupID] = &group
	n.mu.Unlock()

	return nil
}

// startPeriodicSync starts a goroutine to sync all groups periodically
func (n *Node) startPeriodicSync() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			n.mu.RLock()
			groupIDs := make([]string, 0, len(n.Groups))
			for id := range n.Groups {
				groupIDs = append(groupIDs, id)
			}
			n.mu.RUnlock()
			for _, id := range groupIDs {
				n.SyncGroup(id)
			}
		}
	}()
}

// GetBalances returns the balances for a user in a group
func (n *Node) GetBalances(groupID, userID string) map[string]float64 {
	n.mu.RLock()
	group, exists := n.Groups[groupID]
	n.mu.RUnlock()
	if !exists {
		return nil
	}
	balances := make(map[string]float64)
	for payer, counter := range group.Balances[userID] {
		balances[payer] = float64(counter.Value()) / 100.0
	}
	return balances
}

func main() {
	node := NewNode()

	// Setup HTTP routes
	http.HandleFunc("/createGroup", node.handleCreateGroup)
	http.HandleFunc("/joinGroup", node.handleJoinGroup)
	http.HandleFunc("/addExpense", node.handleAddExpense)
	http.HandleFunc("/getBalances", node.handleGetBalances)

	// Start periodic sync
	node.startPeriodicSync()

	// Start server in a goroutine
	go func() {
		log.Println("Starting server on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Demo usage
	err := node.CreateGroup("group1", "Trip to Paris", "alice")
	if err != nil {
		log.Fatal(err)
	}

	err = node.JoinGroup("group1", "bob")
	if err != nil {
		log.Fatal(err)
	}

	expense := models.Expense{
		ID:           "exp1",
		Amount:       100.0,
		Description:  "Dinner",
		Payer:        "alice",
		Participants: []string{"alice", "bob"},
	}
	err = node.AddExpense("group1", expense)
	if err != nil {
		log.Fatal(err)
	}

	balances := node.GetBalances("group1", "bob")
	fmt.Printf("Bob's balances: %v\n", balances)

	// Keep running
	time.Sleep(10 * time.Second)
}
