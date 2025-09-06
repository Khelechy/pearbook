package node

import (
	"sync"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/khelechy/pearbook/models"
	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/dht"
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


// CreateGroup creates a new group
func (n *Node) CreateGroup(groupID, name string, creator string) error {
	group := &models.Group{
		ID:       groupID,
		Name:     name,
		Members:  crdt.NewORSet(),
		Expenses: crdt.NewORMap(),
		Balances: make(map[string]map[string]*crdt.PNCounter),
	}

	group.Members.Add(creator, uuid.New().String()) // unique tag

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
		group.Members.Add(userID, uuid.New().String()) // unique tag
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

	group.Expenses.Put(expense.ID, expense, uuid.New().String())

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

	var remoteGroup models.Group
	json.Unmarshal([]byte(val), &remoteGroup)

	n.mu.Lock()
	localGroup, exists := n.Groups[groupID]
	if !exists {
		// If no local group, just set it
		n.Groups[groupID] = &remoteGroup
	} else {
		// Merge CRDTs
		localGroup.Members.Merge(remoteGroup.Members)
		localGroup.Expenses.Merge(remoteGroup.Expenses)
		// For balances, merge each PN-Counter
		for user, remoteBalances := range remoteGroup.Balances {
			if localGroup.Balances[user] == nil {
				localGroup.Balances[user] = make(map[string]*crdt.PNCounter)
			}
			for payer, remoteCounter := range remoteBalances {
				if localGroup.Balances[user][payer] == nil {
					localGroup.Balances[user][payer] = crdt.NewPNCounter()
				}
				localGroup.Balances[user][payer].Merge(remoteCounter)
			}
		}
		// Update non-CRDT fields
		localGroup.Name = remoteGroup.Name
	}
	n.mu.Unlock()

	return nil
}

// StartPeriodicSync starts a goroutine to sync all groups periodically
func (n *Node) StartPeriodicSync() {
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