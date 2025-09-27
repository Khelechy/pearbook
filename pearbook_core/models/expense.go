package models

import "time"

// Expense represents an expense
type Expense struct {
	ID           string             `json:"id"`
	Amount       float64            `json:"amount"`
	Description  string             `json:"description"`
	Payer        string             `json:"payer"`
	Participants []string           `json:"participants"`
	Splits       map[string]float64 `json:"splits"` // user -> amount owed
	Timestamp    time.Time          `json:"timestamp"`
	Signature    []byte             `json:"signature"` // Digital signature from payer
}

// SignedOperation represents a signed operation request
type SignedOperation struct {
	Operation string                 `json:"operation"` // "join", "approve", "add_expense", etc.
	GroupID   string                 `json:"group_id"`
	UserID    string                 `json:"user_id"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`      // Operation-specific data
	Signature []byte                 `json:"signature"` // Digital signature
}

// BalanceInfo represents balance information with user details
type BalanceInfo struct {
	UserID   string  `json:"user_id"`
	UserName string  `json:"user_name"`
	Amount   float64 `json:"amount"`
}

// BalancesResponse represents the response for GetBalances
type BalancesResponse struct {
	Balances []BalanceInfo `json:"balances"`
}
