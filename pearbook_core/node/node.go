package node

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/khelechy/pearbook/crdt"
	"github.com/khelechy/pearbook/dht"
	"github.com/khelechy/pearbook/models"

	kdht "github.com/libp2p/go-libp2p-kad-dht"
)

// Node represents a peer node
type Node struct {
	ID             string
	DHT            *dht.SimulatedDHT
	KDHT           *kdht.IpfsDHT
	shards         []map[string]*models.Group // sharded local cache
	shardLocks     []sync.RWMutex             // per-shard locks
	numShards      int
	cachedBalances map[string]map[string]float64 // precomputed balances
	balanceMu      sync.RWMutex
}

// getShardIndex returns the shard index for a groupID
func (n *Node) getShardIndex(groupID string) int {
	hash := fnv.New32a()
	hash.Write([]byte(groupID))
	return int(hash.Sum32()) % n.numShards
}

// GetGroups returns a copy of all groups for testing purposes
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

// compressData compresses data using gzip
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(data)
	if err != nil {
		return nil, err
	}
	gz.Close()
	return buf.Bytes(), nil
}

// decompressData decompresses gzip data
func decompressData(data []byte) ([]byte, error) {
	buf := bytes.NewReader(data)
	gz, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
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
		ID:             uuid.New().String(),
		DHT:            dht.NewSimulatedDHT(),
		shards:         shards,
		shardLocks:     shardLocks,
		numShards:      numShards,
		cachedBalances: make(map[string]map[string]float64),
	}
}

func NewNodeWithKDHT(kadDHT *kdht.IpfsDHT) *Node {
	numShards := 16
	shards := make([]map[string]*models.Group, numShards)
	shardLocks := make([]sync.RWMutex, numShards)
	for i := range shards {
		shards[i] = make(map[string]*models.Group)
	}
	return &Node{
		ID:             uuid.New().String(),
		DHT:            dht.NewSimulatedDHT(),
		KDHT:           kadDHT,
		shards:         shards,
		shardLocks:     shardLocks,
		numShards:      numShards,
		cachedBalances: make(map[string]map[string]float64),
	}
}

// CreateGroup creates a new group
func (n *Node) CreateGroup(ctx context.Context, groupID, name string, creator string) error {
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

	if n.KDHT != nil {
		data, _ := json.Marshal(group)
		key := fmt.Sprintf("group:%s", groupID)
		compressed, err := compressData(data)
		if err != nil {
			return err
		}
		err = n.KDHT.PutValue(ctx, "/namespace/"+key, compressed)
		return err
	}
	return nil
}

// JoinGroup joins an existing group
func (n *Node) JoinGroup(ctx context.Context, groupID, userID string) error {
	if n.KDHT == nil {
		return fmt.Errorf("group not found")
	}

	key := fmt.Sprintf("group:%s", groupID)
	val, err := n.KDHT.GetValue(ctx, "/namespace/"+key)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	decompressed, err := decompressData([]byte(val))
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	var group models.Group
	err = json.Unmarshal(decompressed, &group)
	if err != nil {
		return fmt.Errorf("failed to unmarshal group: %w", err)
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

	data, _ := json.Marshal(group)
	compressed, err := compressData(data)
	if err != nil {
		return err
	}
	err = n.KDHT.PutValue(ctx, "/namespace/"+key, compressed)
	if err != nil {
		return err
	}

	return nil
}

// AddExpense adds an expense to a group
func (n *Node) AddExpense(ctx context.Context, groupID string, expense models.Expense) error {
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].Lock()
	group, exists := n.shards[shardIndex][groupID]

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
		group.Balances[user][expense.Payer].Increment(n.ID, int64(owed*100))
	}

	data, err := json.Marshal(group)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("group:%s", groupID)
	if n.KDHT == nil {
		return fmt.Errorf("DHT not available")
	}

	compressed, err := compressData(data)
	if err != nil {
		return err
	}
	err = n.KDHT.PutValue(ctx, "/namespace/"+key, compressed)
	if err != nil {
		return err
	}

	// Invalidate balance cache for all participants
	n.balanceMu.Lock()
	for _, p := range expense.Participants {
		cacheKey := fmt.Sprintf("%s:%s", groupID, p)
		delete(n.cachedBalances, cacheKey)
	}
	payerCacheKey := fmt.Sprintf("%s:%s", groupID, expense.Payer)
	delete(n.cachedBalances, payerCacheKey)
	n.balanceMu.Unlock()
	n.shardLocks[shardIndex].Unlock()

	return nil
}

// GetBalances returns the balances for a user in a group, using cache
func (n *Node) GetBalances(groupID, userID string) map[string]float64 {
	cacheKey := fmt.Sprintf("%s:%s", groupID, userID)
	n.balanceMu.Lock() // Exclusive lock for entire operation
	if balances, exists := n.cachedBalances[cacheKey]; exists {
		n.balanceMu.Unlock()
		return balances
	}
	// Compute and set cache
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()
	if !exists {
		n.balanceMu.Unlock()
		return nil
	}
	balances := make(map[string]float64)
	for payer, counter := range group.Balances[userID] {
		balances[payer] = float64(counter.Value()) / 100.0
	}
	n.cachedBalances[cacheKey] = balances
	n.balanceMu.Unlock()
	return balances
}

// SyncGroup syncs group data from DHT
func (n *Node) SyncGroup(ctx context.Context, groupID string) error {
	fmt.Printf("Syncing group %s\n", groupID)

	if n.KDHT == nil {
		return fmt.Errorf("DHT not available")
	}

	key := fmt.Sprintf("group:%s", groupID)
	val, err := n.KDHT.GetValue(ctx, "/namespace/"+key)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	decompressed, err := decompressData([]byte(val))
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}
	var remoteGroup models.Group
	json.Unmarshal(decompressed, &remoteGroup)

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

		// Print new local members after merge
		localMembers := localGroup.Members.Elements()
		fmt.Printf("Local members after merge for group %s: %v\n", groupID, localMembers)

		localGroup.Expenses.Merge(remoteGroup.Expenses)
		// For balances, merge each PN-Counter concurrently
		var wg sync.WaitGroup
		for user, remoteBalances := range remoteGroup.Balances {
			wg.Add(1)
			go func(u string, rb map[string]*crdt.PNCounter) {
				defer wg.Done()
				if localGroup.Balances[u] == nil {
					localGroup.Balances[u] = make(map[string]*crdt.PNCounter)
				}
				for payer, remoteCounter := range rb {
					if localGroup.Balances[u][payer] == nil {
						localGroup.Balances[u][payer] = crdt.NewPNCounter()
					}
					localGroup.Balances[u][payer].Merge(remoteCounter)
				}

				fmt.Printf("Merged balances for user %s\n", u)
			}(user, remoteBalances)
		}
		wg.Wait()
		// Update non-CRDT fields
		localGroup.Name = remoteGroup.Name
		fmt.Printf("Group %s synced (updated)\n", groupID)
	}
	n.shardLocks[shardIndex].Unlock()

	return nil
}

// StartPeriodicSync starts a goroutine to sync all groups periodically with concurrency
func (n *Node) StartPeriodicSync(ctx context.Context, numWorkers int) {
	fmt.Println("Starting periodic group sync")

	// Perform initial sync immediately
	n.performSync(ctx, numWorkers)

	ticker := time.NewTicker(7 * time.Second)
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

// performSync performs the sync logic
func (n *Node) performSync(ctx context.Context, numWorkers int) {
	// Collect groupIDs from all shards
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
	// Use worker pool for concurrent sync
	jobs := make(chan string, len(groupIDs))
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for groupID := range jobs {
				fmt.Printf("Syncing group %s\n", groupID)
				ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
				err := n.SyncGroup(ctxTimeout, groupID)
				cancel()
				if err != nil {
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
