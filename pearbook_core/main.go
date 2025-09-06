package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/khelechy/pearbook/models"
	"github.com/khelechy/pearbook/node"

	"github.com/common-nighthawk/go-figure"
)

// Local wrapper type for HTTP handlers
type server struct {
	*node.Node
}

func (s *server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string `json:"group_id"`
		Name    string `json:"name"`
		Creator string `json:"creator"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.CreateGroup(req.GroupID, req.Name, req.Creator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Group created")
}

func (s *server) handleJoinGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.JoinGroup(req.GroupID, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Joined group")
}

func (s *server) handleAddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string         `json:"group_id"`
		Expense models.Expense `json:"expense"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.AddExpense(req.GroupID, req.Expense)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Expense added")
}

func (s *server) handleGetBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	groupID := r.URL.Query().Get("group_id")
	userID := r.URL.Query().Get("user_id")
	// Sync before getting balances
	s.SyncGroup(groupID)
	balances := s.GetBalances(groupID, userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balances)
}

func main() {
	n := node.NewNode()
	srv := &server{Node: n}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/createGroup", srv.handleCreateGroup)
	mux.HandleFunc("/joinGroup", srv.handleJoinGroup)
	mux.HandleFunc("/addExpense", srv.handleAddExpense)
	mux.HandleFunc("/getBalances", srv.handleGetBalances)

	// Start periodic sync
	n.StartPeriodicSync()

	srvAddr := ":8080"
	server := &http.Server{
		Addr:    srvAddr,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		myfigure := figure.NewColorFigure("PearBook", "puffy", "green", true)
		myfigure.Print()

		log.Println("Starting server on", srvAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Demo usage (uncomment to use)
	// err := srv.CreateGroup("group1", "Trip to Paris", "alice")
	// if err != nil {
	//      log.Fatal(err)
	// }

	// err = srv.JoinGroup("group1", "bob")
	// if err != nil {
	//      log.Fatal(err)
	// }

	// expense := models.Expense{
	//      ID:           "exp1",
	//      Amount:       100.0,
	//      Description:  "Dinner",
	//      Payer:        "alice",
	//      Participants: []string{"alice", "bob"},
	// }
	// err = srv.AddExpense("group1", expense)
	// if err != nil {
	//      log.Fatal(err)
	// }

	// balances := srv.GetBalances("group1", "bob")
	// fmt.Printf("Bob's balances: %v\n", balances)

	// Wait for SIGINT or SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("Server exited gracefully")
}
