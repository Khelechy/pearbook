package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/khelechy/pearbook/pearbook_core/models"
	"github.com/khelechy/pearbook/pearbook_core/node"
)

// Server wraps the node with HTTP handlers
type Server struct {
	*node.Node
}

func (s *Server) HandleCreateGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var signedOp models.SignedOperation
	if err := json.NewDecoder(r.Body).Decode(&signedOp); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode JSON: %v", err), http.StatusBadRequest)
		return
	}
	err := s.CreateGroup(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Group created")
}

func (s *Server) HandleJoinGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var signedOp models.SignedOperation
	if err := json.NewDecoder(r.Body).Decode(&signedOp); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	err := s.JoinGroup(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Join request submitted")
}

func (s *Server) HandleAddExpense(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var signedOp models.SignedOperation
	if err := json.NewDecoder(r.Body).Decode(&signedOp); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	err := s.AddExpense(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Expense added")
}

func (s *Server) HandleGetBalances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract parameters from query string
	groupID := r.URL.Query().Get("group_id")
	userID := r.URL.Query().Get("user_id")

	if groupID == "" || userID == "" {
		http.Error(w, "Missing required parameters: group_id and user_id", http.StatusBadRequest)
		return
	}

	// Create minimal signed operation for read-only request
	signedOp := models.SignedOperation{
		Operation: "get_balances",
		GroupID:   groupID,
		UserID:    userID,
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{},
	}

	balances, err := s.GetBalances(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balances)
}

func (s *Server) HandleGetPendingJoins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract parameters from query string
	groupID := r.URL.Query().Get("group_id")
	userID := r.URL.Query().Get("user_id")

	if groupID == "" || userID == "" {
		http.Error(w, "Missing required parameters: group_id and user_id", http.StatusBadRequest)
		return
	}

	// Create minimal signed operation for read-only request
	signedOp := models.SignedOperation{
		Operation: "get_pending_joins",
		GroupID:   groupID,
		UserID:    userID,
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{},
	}

	pendingJoins, err := s.GetPendingJoins(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingJoins)
}

func (s *Server) HandleApproveJoin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var signedOp models.SignedOperation
	if err := json.NewDecoder(r.Body).Decode(&signedOp); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	err := s.ApproveJoin(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Join request approved")
}
