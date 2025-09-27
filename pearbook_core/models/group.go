package models

import (
	"time"

	"github.com/khelechy/pearbook/pearbook_core/crdt"
)

// Group represents a group of users
type Group struct {
	ID              string
	Name            string
	Members         *crdt.ORMap                           // CRDT Map: userID -> MemberInfo (extended from ORSet)
	Expenses        *crdt.ORMap                           // CRDT Map for expenses
	Balances        map[string]map[string]*crdt.PNCounter // user -> user -> balance CRDT
	GroupKey        []byte                                // Encrypted group master key
	GroupKeyUpdated time.Time                             // Timestamp for LWW resolution
	PendingJoins    *crdt.ORMap                           // CRDT Map: requestID -> JoinRequest
}

// MemberInfo represents information about an approved member
type MemberInfo struct {
	UserID    string    // Internal user ID for operations
	UserName  string    // Display name for UI
	PublicKey []byte    // Member's public key for verification
	JoinedAt  time.Time // When the member was approved
}

// JoinRequest represents a pending join request
type JoinRequest struct {
	RequesterID string            // ID of the requesting peer
	UserName    string            // Display name of the requester
	PublicKey   []byte            // Requester's public key
	Timestamp   time.Time         // When the request was made
	Approvals   map[string][]byte // Map of approverID -> signature
}

// Delta represents a change operation
type Delta struct {
	Type        string // "expense_add", "member_add", "balance_update"
	GroupID     string
	Data        interface{} // Expense, Member, or BalanceChange
	Timestamp   time.Time
	NodeID      string // Originating node
	Version     int    // Logical clock value
	AckRequired bool   // Whether ACK is needed
}

// BalanceChange represents a balance update
type BalanceChange struct {
	User   string
	Payer  string
	Amount int64 // Delta amount (positive/negative)
}
