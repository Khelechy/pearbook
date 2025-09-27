package node

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/khelechy/pearbook/pearbook_core/crdt"
	"github.com/khelechy/pearbook/pearbook_core/dht"
	"github.com/khelechy/pearbook/pearbook_core/models"
	"github.com/khelechy/pearbook/pearbook_core/utils"

	kdht "github.com/libp2p/go-libp2p-kad-dht"
)

// Node represents a peer node
type Node struct {
	ID         string
	KDHT       interface{}                // can be *kdht.IpfsDHT or *dht.SimulatedDHT
	shards     []map[string]*models.Group // sharded local cache
	shardLocks []sync.RWMutex             // per-shard locks
	numShards  int
	crypto     utils.CryptoService // cryptographic service
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
		crypto:     utils.NewCryptoService(),
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
		crypto:     utils.NewCryptoService(),
	}
}

func (n *Node) getShardIndex(groupID string) int {
	hash := fnv.New32a()
	hash.Write([]byte(groupID))
	return int(hash.Sum32()) % n.numShards
}

// CreateGroup creates a new group
func (n *Node) CreateGroup(ctx context.Context, signedOp models.SignedOperation) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "create_group" {
		return fmt.Errorf("invalid operation type for CreateGroup")
	}

	// Extract group data
	groupData, ok := signedOp.Data["group"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid group data in signed operation")
	}

	groupID := groupData["id"].(string)
	name := groupData["name"].(string)

	// Extract user data
	userData, ok := signedOp.Data["user"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data in signed operation")
	}

	userName := userData["user_name"].(string)
	publicKey := userData["public_key"].(string)

	// Decode public key using crypto service
	publicKeyBytes, err := n.crypto.DecodePublicKey(publicKey, signedOp.UserID)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	// Generate group master key (for now, use a simple key - in production use proper crypto)
	groupKey := []byte(fmt.Sprintf("group-key-%s-%d", groupID, time.Now().Unix()))

	group := &models.Group{
		ID:              groupID,
		Name:            name,
		Members:         crdt.NewORMap(),
		Expenses:        crdt.NewORMap(),
		Balances:        make(map[string]map[string]*crdt.PNCounter),
		GroupKey:        groupKey,
		GroupKeyUpdated: time.Now(),
		PendingJoins:    crdt.NewORMap(),
	}

	// Add creator as approved member
	memberInfo := models.MemberInfo{
		UserID:    signedOp.UserID,
		UserName:  userName,
		PublicKey: publicKeyBytes,
		JoinedAt:  time.Now(),
	}
	group.Members.Put(signedOp.UserID, memberInfo, uuid.New().String())

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

	// Register group in global registry
	n.registerGroupInRegistry(ctx, groupID)

	return nil
}

// JoinGroup creates a join request for an existing group with signature verification
func (n *Node) JoinGroup(ctx context.Context, signedOp models.SignedOperation) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "join_group" {
		return fmt.Errorf("invalid operation type for JoinGroup")
	}

	groupID := signedOp.GroupID

	// Check local cache first
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		// Fetch group from DHT if not in local cache
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

		group = &models.Group{}
		err = json.Unmarshal(decompressedData, group)
		if err != nil {
			return fmt.Errorf("failed to unmarshal group data: %w", err)
		}

		// Cache the fetched group locally for future operations
		n.shardLocks[shardIndex].Lock()
		n.shards[shardIndex][groupID] = group
		n.shardLocks[shardIndex].Unlock()
	}

	// Check if already a member
	if _, exists := group.Members.Get(signedOp.UserID); exists {
		return fmt.Errorf("user is already a member of the group")
	}

	// Check if join request already exists
	requestID := fmt.Sprintf("%s:%s", groupID, signedOp.UserID)
	if _, exists := group.PendingJoins.Get(requestID); exists {
		return fmt.Errorf("join request already pending")
	}

	// Extract user data
	userData, ok := signedOp.Data["user"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data in signed operation")
	}

	userName := userData["user_name"].(string)
	publicKey := userData["public_key"].(string)

	// Decode public key using crypto service
	publicKeyBytes, err := n.crypto.DecodePublicKey(publicKey, signedOp.UserID)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	// Create join request
	joinRequest := models.JoinRequest{
		RequesterID: signedOp.UserID,
		UserName:    userName,
		PublicKey:   publicKeyBytes,
		Timestamp:   time.Now(),
		Approvals:   make(map[string][]byte),
	}

	group.PendingJoins.Put(requestID, joinRequest, uuid.New().String())

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
		return fmt.Errorf("failed to put group data in DHT: %w", err)
	}
	return nil
}

// ApproveJoin approves a pending join request with signature verification
func (n *Node) ApproveJoin(ctx context.Context, signedOp models.SignedOperation) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "approve_join" {
		return fmt.Errorf("invalid operation type for ApproveJoin")
	}

	groupID := signedOp.GroupID

	// Check local cache first
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		// Fetch group from DHT if not in local cache
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

		group = &models.Group{}
		err = json.Unmarshal(decompressedData, group)
		if err != nil {
			return fmt.Errorf("failed to unmarshal group data: %w", err)
		}

		// Cache the fetched group locally for future operations
		n.shardLocks[shardIndex].Lock()
		n.shards[shardIndex][groupID] = group
		n.shardLocks[shardIndex].Unlock()
	}

	// Check if approver is a member
	if _, exists := group.Members.Get(signedOp.UserID); !exists {
		return fmt.Errorf("only group members can approve join requests")
	}

	// Get approver's public key for signature verification
	memberInfoInterface, exists := group.Members.Get(signedOp.UserID)
	if !exists {
		return fmt.Errorf("approver not found in group members")
	}

	memberInfoData, err := json.Marshal(memberInfoInterface)
	if err != nil {
		return fmt.Errorf("failed to marshal member info: %w", err)
	}

	var memberInfo models.MemberInfo
	err = json.Unmarshal(memberInfoData, &memberInfo)
	if err != nil {
		return fmt.Errorf("failed to unmarshal member info: %w", err)
	}

	// Verify signature
	opData := n.crypto.CreateOperationData(signedOp.Operation, signedOp.GroupID, signedOp.UserID, signedOp.Timestamp, signedOp.Data)
	err = n.crypto.VerifySignature(memberInfo.PublicKey, opData, signedOp.Signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Get the request ID from the signed operation data
	requestID, ok := signedOp.Data["request_id"].(string)
	if !ok {
		return fmt.Errorf("request_id not found in signed operation data")
	}

	// Parse the requester ID from the request ID (format: "groupID:userID")
	parts := strings.Split(requestID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid request_id format")
	}
	requesterID := parts[1]

	// Check if requester is already a member
	if _, alreadyMember := group.Members.Get(requesterID); alreadyMember {
		return fmt.Errorf("user is already a member of the group")
	}

	// Get the join request
	joinRequestInterface, exists := group.PendingJoins.Get(requestID)
	if !exists {
		return fmt.Errorf("join request not found")
	}

	// Convert interface{} to JoinRequest
	joinRequestData, err := json.Marshal(joinRequestInterface)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %w", err)
	}

	var joinRequest models.JoinRequest
	err = json.Unmarshal(joinRequestData, &joinRequest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal join request: %w", err)
	}

	// Check if already approved by this member
	if _, alreadyApproved := joinRequest.Approvals[signedOp.UserID]; alreadyApproved {
		return fmt.Errorf("already approved by this member")
	}

	// Add approval (use the signature from the signed operation)
	joinRequest.Approvals[signedOp.UserID] = signedOp.Signature

	// Check if we have enough approvals (single approval required)
	requiredApprovals := 1

	if len(joinRequest.Approvals) >= requiredApprovals {
		// Add member to group
		memberInfo := models.MemberInfo{
			UserID:    joinRequest.RequesterID,
			UserName:  joinRequest.UserName,
			PublicKey: joinRequest.PublicKey,
			JoinedAt:  time.Now(),
		}
		group.Members.Put(joinRequest.RequesterID, memberInfo, uuid.New().String())

		// Initialize balances for new member
		group.Balances[joinRequest.RequesterID] = make(map[string]*crdt.PNCounter)

		// Remove from pending joins
		group.PendingJoins.Remove(requestID, "")
	} else {
		// Update the pending request with new approval
		group.PendingJoins.Put(requestID, joinRequest, uuid.New().String())
	}

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
		return fmt.Errorf("failed to put group data in DHT: %w", err)
	}
	return nil
}

// GetPendingJoins returns all pending join requests for a group
func (n *Node) GetPendingJoins(ctx context.Context, signedOp models.SignedOperation) (map[string]models.JoinRequest, error) {
	if n.KDHT == nil {
		return nil, fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "get_pending_joins" {
		return nil, fmt.Errorf("invalid operation type for GetPendingJoins")
	}

	groupID := signedOp.GroupID

	// Sync with latest data from DHT to ensure we have current pending joins
	if err := n.SyncGroup(ctx, groupID); err != nil {
		return nil, fmt.Errorf("failed to sync group: %w", err)
	}

	// Check local cache first
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		// Fetch group from DHT if not in local cache
		key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
		var val []byte
		var err error
		if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
			val, err = ipfs.GetValue(ctx, key)
		} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
			val, err = sim.GetValue(ctx, key)
		} else {
			return nil, fmt.Errorf("unsupported DHT type")
		}
		if err != nil {
			return nil, fmt.Errorf("group not found: %w", err)
		}

		decompressedData, err := utils.DecompressData([]byte(val))
		if err != nil {
			return nil, fmt.Errorf("failed to decompress group data: %w", err)
		}

		group = &models.Group{}
		err = json.Unmarshal(decompressedData, group)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal group data: %w", err)
		}

		// Cache the fetched group locally for future operations
		n.shardLocks[shardIndex].Lock()
		n.shards[shardIndex][groupID] = group
		n.shardLocks[shardIndex].Unlock()
	}

	// Check if requester is a member
	if _, exists := group.Members.Get(signedOp.UserID); !exists {
		return nil, fmt.Errorf("only group members can view pending joins")
	}

	// Extract pending joins - merge approvals from concurrent versions
	pendingJoins := make(map[string]models.JoinRequest)
	for _, requestID := range group.PendingJoins.Keys() {
		// Get all versions for this requestID and merge approvals
		mergedRequest := models.JoinRequest{}
		first := true

		if values := group.PendingJoins.GetAll(requestID); values != nil {
			for _, v := range values {
				// Handle both in-memory structs and JSON-unmarshaled maps
				joinRequestData, err := json.Marshal(v)
				if err != nil {
					continue
				}
				var joinRequest models.JoinRequest
				err = json.Unmarshal(joinRequestData, &joinRequest)
				if err != nil {
					continue
				}

				if first {
					mergedRequest = joinRequest
					first = false
				} else {
					// Merge approvals
					for approver, sig := range joinRequest.Approvals {
						mergedRequest.Approvals[approver] = sig
					}
				}
			}
		}

		if !first { // We found at least one JoinRequest
			pendingJoins[requestID] = mergedRequest
		}
	}

	return pendingJoins, nil
}

// AddExpense adds an expense to a group with signature verification
func (n *Node) AddExpense(ctx context.Context, signedOp models.SignedOperation) error {
	if n.KDHT == nil {
		return fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "add_expense" {
		return fmt.Errorf("invalid operation type for AddExpense")
	}

	groupID := signedOp.GroupID

	// Check local cache first
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		// Fetch group from DHT if not in local cache
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

		group = &models.Group{}
		err = json.Unmarshal(decompressedData, group)
		if err != nil {
			return fmt.Errorf("failed to unmarshal group data: %w", err)
		}

		// Cache the fetched group locally for future operations
		n.shardLocks[shardIndex].Lock()
		n.shards[shardIndex][groupID] = group
		n.shardLocks[shardIndex].Unlock()
	}

	// Check if payer is an approved member
	memberInfoInterface, exists := group.Members.Get(signedOp.UserID)
	if !exists {
		return fmt.Errorf("user is not an approved member")
	}

	memberInfoData, err := json.Marshal(memberInfoInterface)
	if err != nil {
		return fmt.Errorf("failed to marshal member info: %w", err)
	}

	var memberInfo models.MemberInfo
	err = json.Unmarshal(memberInfoData, &memberInfo)
	if err != nil {
		return fmt.Errorf("failed to unmarshal member info: %w", err)
	}

	// Verify signature
	opData := n.crypto.CreateOperationData(signedOp.Operation, signedOp.GroupID, signedOp.UserID, signedOp.Timestamp, signedOp.Data)
	err = n.crypto.VerifySignature(memberInfo.PublicKey, opData, signedOp.Signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Extract expense data
	expenseData, ok := signedOp.Data["expense"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid expense data in signed operation")
	}

	expense := models.Expense{
		ID:          expenseData["id"].(string),
		Amount:      expenseData["amount"].(float64),
		Description: expenseData["description"].(string),
		Payer:       signedOp.UserID,
		Timestamp:   time.Unix(signedOp.Timestamp, 0),
		Signature:   signedOp.Signature,
	}

	// Handle participants
	if participants, ok := expenseData["participants"].([]interface{}); ok {
		for _, p := range participants {
			expense.Participants = append(expense.Participants, p.(string))
		}
	}

	// Verify all participants are approved members
	for _, participant := range expense.Participants {
		if _, exists := group.Members.Get(participant); !exists {
			return fmt.Errorf("all participants must be approved group members")
		}
	}

	n.shardLocks[shardIndex].Lock()

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

	defer n.shardLocks[shardIndex].Unlock()

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
		return fmt.Errorf("failed to put group data in DHT: %w", err)
	}

	return nil
}

// GetBalances returns the balances for a user in a group
func (n *Node) GetBalances(ctx context.Context, signedOp models.SignedOperation) (map[string]map[string]interface{}, error) {
	if n.KDHT == nil {
		return nil, fmt.Errorf("KDHT not initialized")
	}

	// Verify operation type
	if signedOp.Operation != "get_balances" {
		return nil, fmt.Errorf("invalid operation type for GetBalances")
	}

	groupID := signedOp.GroupID

	// Check local cache first
	shardIndex := n.getShardIndex(groupID)
	n.shardLocks[shardIndex].RLock()
	group, exists := n.shards[shardIndex][groupID]
	n.shardLocks[shardIndex].RUnlock()

	if !exists {
		// Fetch group from DHT if not in local cache
		key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
		var val []byte
		var err error
		if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
			val, err = ipfs.GetValue(ctx, key)
		} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
			val, err = sim.GetValue(ctx, key)
		} else {
			return nil, fmt.Errorf("unsupported DHT type")
		}
		if err != nil {
			return nil, fmt.Errorf("group not found: %w", err)
		}

		decompressedData, err := utils.DecompressData([]byte(val))
		if err != nil {
			return nil, fmt.Errorf("failed to decompress group data: %w", err)
		}

		group = &models.Group{}
		err = json.Unmarshal(decompressedData, group)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal group data: %w", err)
		}

		// Cache the fetched group locally for future operations
		n.shardLocks[shardIndex].Lock()
		n.shards[shardIndex][groupID] = group
		n.shardLocks[shardIndex].Unlock()
	}

	// Check if user is an approved member
	if _, exists := group.Members.Get(signedOp.UserID); !exists {
		return nil, fmt.Errorf("user is not an approved member")
	}

	balances := make(map[string]map[string]interface{})
	for payer, counter := range group.Balances[signedOp.UserID] {
		// Get payer's user name
		payerInfoInterface, exists := group.Members.Get(payer)
		payerName := payer // fallback to user ID if name not found
		if exists {
			payerInfoData, err := json.Marshal(payerInfoInterface)
			if err == nil {
				var payerInfo models.MemberInfo
				if err := json.Unmarshal(payerInfoData, &payerInfo); err == nil {
					payerName = payerInfo.UserName
				}
			}
		}

		balances[payer] = map[string]interface{}{
			"user_id":   payer,
			"user_name": payerName,
			"amount":    float64(counter.Value()) / 100.0,
		}
	}
	return balances, nil
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
		localGroup.PendingJoins.Merge(remoteGroup.PendingJoins)
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
		if remoteGroup.GroupKeyUpdated.After(localGroup.GroupKeyUpdated) {
			localGroup.GroupKey = remoteGroup.GroupKey
			localGroup.GroupKeyUpdated = remoteGroup.GroupKeyUpdated
		}
		fmt.Printf("Group %s synced (merged)\n", groupID)
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
	// First, discover new groups from the registry
	n.discoverGroupsFromRegistry(ctx)

	// Then sync all known groups
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

// registerGroupInRegistry adds a group to the global group registry
func (n *Node) registerGroupInRegistry(ctx context.Context, groupID string) {
	if n.KDHT == nil {
		return
	}

	registryKey := "/namespace/group-registry"

	// Get current registry
	var groupIDs []string
	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		if val, err := ipfs.GetValue(ctx, registryKey); err == nil {
			if decompressed, err := utils.DecompressData(val); err == nil {
				json.Unmarshal(decompressed, &groupIDs)
			}
		}
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		if val, err := sim.GetValue(ctx, registryKey); err == nil {
			if decompressed, err := utils.DecompressData(val); err == nil {
				json.Unmarshal(decompressed, &groupIDs)
			}
		}
	}

	// Add new group if not already present
	found := false
	for _, id := range groupIDs {
		if id == groupID {
			found = true
			break
		}
	}
	if !found {
		groupIDs = append(groupIDs, groupID)
	}

	// Update registry
	if data, err := json.Marshal(groupIDs); err == nil {
		if compressed, err := utils.CompressData(data); err == nil {
			if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
				ipfs.PutValue(ctx, registryKey, compressed)
			} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
				sim.PutValue(ctx, registryKey, compressed)
			}
		}
	}
}

// discoverGroupsFromRegistry fetches the global group registry and adds unknown groups to local cache
func (n *Node) discoverGroupsFromRegistry(ctx context.Context) {
	if n.KDHT == nil {
		return
	}

	registryKey := "/namespace/group-registry"

	var groupIDs []string
	if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
		if val, err := ipfs.GetValue(ctx, registryKey); err == nil {
			if decompressed, err := utils.DecompressData(val); err == nil {
				json.Unmarshal(decompressed, &groupIDs)
			}
		}
	} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
		if val, err := sim.GetValue(ctx, registryKey); err == nil {
			if decompressed, err := utils.DecompressData(val); err == nil {
				json.Unmarshal(decompressed, &groupIDs)
			}
		}
	}

	// Add any unknown groups to local cache by fetching them
	for _, groupID := range groupIDs {
		shardIndex := n.getShardIndex(groupID)
		n.shardLocks[shardIndex].RLock()
		_, exists := n.shards[shardIndex][groupID]
		n.shardLocks[shardIndex].RUnlock()

		if !exists {
			// Fetch group and add to cache
			key := fmt.Sprintf("/namespace/%s/%s", "group", groupID)
			if ipfs, ok := n.KDHT.(*kdht.IpfsDHT); ok {
				if val, err := ipfs.GetValue(ctx, key); err == nil {
					if decompressed, err := utils.DecompressData(val); err == nil {
						var group models.Group
						if err := json.Unmarshal(decompressed, &group); err == nil {
							n.shardLocks[shardIndex].Lock()
							n.shards[shardIndex][groupID] = &group
							n.shardLocks[shardIndex].Unlock()
							fmt.Printf("Discovered new group: %s\n", groupID)
						}
					}
				}
			} else if sim, ok := n.KDHT.(*dht.SimulatedDHT); ok {
				if val, err := sim.GetValue(ctx, key); err == nil {
					if decompressed, err := utils.DecompressData(val); err == nil {
						var group models.Group
						if err := json.Unmarshal(decompressed, &group); err == nil {
							n.shardLocks[shardIndex].Lock()
							n.shards[shardIndex][groupID] = &group
							n.shardLocks[shardIndex].Unlock()
							fmt.Printf("Discovered new group: %s\n", groupID)
						}
					}
				}
			}
		}
	}
}

// SetCryptoService sets the crypto service (for testing purposes)
func (n *Node) SetCryptoService(cryptoService utils.CryptoService) {
	n.crypto = cryptoService
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
