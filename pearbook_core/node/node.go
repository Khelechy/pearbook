package node

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/dht"
	"github.com/khelechy/pearbook/models"
	"github.com/khelechy/pearbook/utils"

	kdht "github.com/libp2p/go-libp2p-kad-dht"
)

// Node represents a peer node
type Node struct {
	ID         string
	KDHT       interface{}                // can be *kdht.IpfsDHT or *dht.SimulatedDHT
	shards     []map[string]*models.Group // sharded local cache
	shardLocks []sync.RWMutex             // per-shard locks
	numShards  int
}

// NewNode creates a new node
func NewNode() *Node {
	numShards := 16
	shards := make([]map[string]*models.Group, numShards)
	shardLocks := make([]sync.RWMutex, numShards)
	for i := range shards {
		shards[i] = make(map[string]*models.Group)
	}
	return &Node{
		ID:         uuid.New().String(),
		KDHT:       nil,
		shards:     shards,
		shardLocks: shardLocks,
		numShards:  numShards,
	}
}

func NewNodeWithKDHT(kadDHT interface{}) *Node {
	numShards := 16
	shards := make([]map[string]*models.Group, numShards)
	shardLocks := make([]sync.RWMutex, numShards)
	for i := range shards {
		shards[i] = make(map[string]*models.Group)
	}
	return &Node{
		ID:         uuid.New().String(),
		KDHT:       kadDHT,
		shards:     shards,
		shardLocks: shardLocks,
		numShards:  numShards,
	}
}

func (n *Node) getShardIndex(groupID string) int {
	hash := fnv.New32a()
	hash.Write([]byte(groupID))
	return int(hash.Sum32()) % n.numShards
}

// CreateGroup creates a new group
func (n *Node) CreateGroup(ctx context.Context, groupID, name string, creator string) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	group := &models.Group{
		ID:       groupID,
		Name:     name,
		Members:  crdt.NewORSet(),
		Expenses: crdt.NewORMap(),
		Balances: make(map[string]map[string]*crdt.PNCounter),
	}

	group.Members.Add(creator, uuid.New().String()) // unique tag

	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].Lock()
	n.shards[shardIndex][groupID] = group
	n.shardLocks[shardIndex].Unlock()

	data, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal group data: %w", err)
	}

	compressedData, err := utils.CompressData(data)
	if err != nil {
		return fmt.Errorf("failed to compress group data: %w", err)
	}

	key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)

	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		err = ipfs.PutValue(ctx, key, compressedData)
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		err = sim.PutValue(ctx, key, compressedData)
	} else {
		return fmt.Errorf("unsupported DHT type")
	}
	if err != nil {
		return err
	}

	return nil
}

// JoinGroup joins an existing group
func (n *Node) JoinGroup(ctx context.Context, groupID, userID string) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	// Fetch group from DHT

	key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
	var val []byte
	var err error
	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		val, err = ipfs.GetValue(ctx, key)
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		val, err = sim.GetValue(ctx, key)
	} else {
		return fmt.Errorf("unsupported DHT type")
	}
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	decompressedData, err := utils.DecompressData([]byte(val))
	if err != nil {
		return fmt.Errorf("failed to decompress group data: %w", err)
	}

	var group models.Group
	err = json.Unmarshal(decompressedData, &group)
	if err != nil {
		return fmt.Errorf("failed to unmarshal group data: %w", err)
	}

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

	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].Lock()
	n.shards[shardIndex][groupID] = &group
	n.shardLocks[shardIndex].Unlock()

	data, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal group data: %w", err)
	}

	compressedData, err := utils.CompressData(data)
	if err != nil {
		return fmt.Errorf("failed to compress group data: %w", err)
	}

	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		err = ipfs.PutValue(ctx, key, compressedData)
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		err = sim.PutValue(ctx, key, compressedData)
	} else {
		return fmt.Errorf("unsupported DHT type")
	}
	if err != nil {
		return fmt.Errorf("failed to put group data in DHT: %w", err)
	}
	return nil
}

// AddExpense adds an expense to a group
func (n *Node) AddExpense(ctx context.Context, groupID string, expense models.Expense) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].Lock()
	group, exists := n.shards[shardIndex][groupID]

	if !exists {
		n.shardLocks[shardIndex].Unlock()
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
		group.Balances[user][expense.Payer].Increment(n.ID, int64(owed*100))
	}

	data, err := json.Marshal(group)
	if err != nil {
		n.shardLocks[shardIndex].Unlock()
		return fmt.Errorf("failed to marshal group data: %w", err)
	}

	compressedData, err := utils.CompressData(data)
	if err != nil {
		n.shardLocks[shardIndex].Unlock()
		return fmt.Errorf("failed to compress group data: %w", err)
	}

	key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
	if n.KDHT == nil {
		n.shardLocks[shardIndex].Unlock()
		return fmt.Errorf("DHT not available")
	}
	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		err = ipfs.PutValue(ctx, key, compressedData)
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		err = sim.PutValue(ctx, key, compressedData)
	} else {
		n.shardLocks[shardIndex].Unlock()
		return fmt.Errorf("unsupported DHT type")
	}
	if err != nil {
		n.shardLocks[shardIndex].Unlock()
		return fmt.Errorf("failed to put group data in DHT: %w", err)
	}

	n.shardLocks[shardIndex].Unlock()
	return nil
}

// GetBalances returns the balances for a user in a group
func (n *Node) GetBalances(groupID, userID string) map[string]float64 {
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		return nil
	}
	balances := make(map[string]float64)
	for payer, counter := range group.Balances[userID] {
		balances[payer] = float64(counter.Value()) / 100.0
	}
	return balances
}

// SyncGroup syncs group data from DHT
func (n *Node) SyncGroup(ctx context.Context, groupID string) error {
	fmt.Println("Syncing group:", groupID)

	if n.KDHT == nil {
		return fmt.Errorf("DHT not available")
	}

	key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
	var val []byte
	var err error
	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		val, err = ipfs.GetValue(ctx, key)
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		val, err = sim.GetValue(ctx, key)
	} else {
		return fmt.Errorf("unsupported DHT type")
	}
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	decompressedData, err := utils.DecompressData([]byte(val))
	if err != nil {
		return fmt.Errorf("failed to decompress group data: %w", err)
	}

	var remoteGroup models.Group
	err = json.Unmarshal(decompressedData, &remoteGroup)
	if err != nil {
		return fmt.Errorf("failed to unmarshal group data: %w", err)
	}

	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].Lock()
	localGroup, exists := n.shards[shardIndex][groupID]

	if !exists {
		// If no local group, just set it
		n.shards[shardIndex][groupID] = &remoteGroup
		fmt.Printf("Group %s synced (new)\n", groupID)
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
		fmt.Printf("Group %s synced (updated)\n", groupID)
	}
	n.shardLocks[shardIndex].Unlock()

	return nil
}

// StartPeriodicSync starts a goroutine to sync all groups periodically
func (n *Node) StartPeriodicSync(ctx context.Context, numWorkers int) {
	fmt.Println("Starting periodic sync...")

	// Perform initial sync immediately
	n.performSync(ctx, numWorkers)

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				n.performSync(ctx, numWorkers)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (n *Node) performSync(ctx context.Context, numWorkers int) {
	var groupIDs []string
	for i := 0; i < n.numShards; i++ {
		n.shardLocks[i].RLock()
		for id := range n.shards[i] {
			groupIDs = append(groupIDs, id)
		}
		n.shardLocks[i].RUnlock()
	}

	if len(groupIDs) == 0 {
		fmt.Println("No groups to sync")
		return
	}

	// Create a worker pool
	jobs := make(chan string, len(groupIDs))
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for groupID := range jobs {
				if err := n.SyncGroup(ctx, groupID); err != nil {
					fmt.Printf("Error syncing group %s: %v\n", groupID, err)
				}
			}
		}()
	}

	for _, id := range groupIDs {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
}

// GetGroups returns a copy of all groups in the local cache
func (n *Node) GetGroups() map[string]*models.Group {
	groups := make(map[string]*models.Group)
	for i := 0; i < n.numShards; i++ {
		n.shardLocks[i].RLock()
		for id, group := range n.shards[i] {
			groups[id] = group
		}
		n.shardLocks[i].RUnlock()
	}
	return groups
}
