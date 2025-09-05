package models

// Expense represents an expense
type Expense struct {
	ID           string
	Amount       float64
	Description  string
	Payer        string
	Participants []string
	Splits       map[string]float64 // user -> amount owed
}
