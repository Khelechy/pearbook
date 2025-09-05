package models

import "github.com/khelechy/pearbook/crdt"

// Group represents a group of users
type Group struct {
	ID       string
	Name     string
	Members  *crdt.ORSet                           // CRDT Set
	Expenses *crdt.ORMap                           // CRDT Map for expenses
	Balances map[string]map[string]*crdt.PNCounter // user -> user -> balance CRDT
}
