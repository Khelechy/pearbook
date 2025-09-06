package models

// Expense represents an expense
type Expense struct {
	ID           string             `json:"id"`
	Amount       float64            `json:"amount"`
	Description  string             `json:"description"`
	Payer        string             `json:"payer"`
	Participants []string           `json:"participants"`
	Splits       map[string]float64 `json:"splits"` // user -> amount owed
}
