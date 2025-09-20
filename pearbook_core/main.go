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

	"github.com/khelechy/pearbook/dht"
	"github.com/khelechy/pearbook/models"
	"github.com/khelechy/pearbook/node"

	"github.com/common-nighthawk/go-figure"
	"github.com/joho/godotenv"
)

// Local wrapper type for HTTP handlers
type server struct {
	*node.Node
}

func (s *server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
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
	err := s.CreateGroup(ctx, signedOp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Group created")
}

func (s *server) handleJoinGroup(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleAddExpense(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleGetBalances(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleGetPendingJoins(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleApproveJoin(w http.ResponseWriter, r *http.Request) {
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

func main() {
	godotenv.Load()

	protocol := os.Getenv("DHT_PROTOCOL")
	bootstrapPeer := os.Getenv("DHT_BOOTSTRAP_PEER")
	bootstrapSetupPort := os.Getenv("DHT_BOOTSTRAP_SETUP_PORT")
	apiPort := os.Getenv("API_PORT")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup libp2p DHT and host
	kadDHT, host, err := dht.SetupLibp2p(ctx, bootstrapPeer, bootstrapSetupPort, protocol)
	if err != nil {
		log.Fatalf("Failed to setup libp2p: %v", err)
	}
	defer func() {
		_ = host.Close()
	}()

	n := node.NewNodeWithKDHT(kadDHT)
	srv := &server{Node: n}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/createGroup", srv.handleCreateGroup)
	mux.HandleFunc("/joinGroup", srv.handleJoinGroup)
	mux.HandleFunc("/addExpense", srv.handleAddExpense)
	mux.HandleFunc("/getBalances", srv.handleGetBalances)
	mux.HandleFunc("/getPendingJoins", srv.handleGetPendingJoins)
	mux.HandleFunc("/approveJoin", srv.handleApproveJoin)

	srvAddr := fmt.Sprintf(":%s", apiPort)
	httpServer := &http.Server{
		Addr:    srvAddr,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		myfigure := figure.NewColorFigure("PearBook", "puffy", "green", true)
		myfigure.Print()

		log.Println("Starting server on", srvAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Start periodic sync
	n.StartPeriodicSync(ctx, 2)

	// Wait for SIGINT or SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")

	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	if err := httpServer.Shutdown(ctxTimeout); err != nil {
		log.Fatalf("HTTP Server Shutdown Failed:%+v", err)
	}
	log.Println("HTTP server exited gracefully")

	// Gracefully shutdown libp2p host
	if err := host.Close(); err != nil {
		log.Printf("Error closing libp2p host: %v", err)
	} else {
		log.Println("libp2p host closed gracefully")
	}
}
