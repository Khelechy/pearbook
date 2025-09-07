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
	var req struct {
		GroupID string `json:"group_id"`
		Name    string `json:"name"`
		Creator string `json:"creator"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.CreateGroup(ctx, req.GroupID, req.Name, req.Creator)
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
	var req struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.JoinGroup(ctx, req.GroupID, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Joined group")
}

func (s *server) handleAddExpense(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID string         `json:"group_id"`
		Expense models.Expense `json:"expense"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	err := s.AddExpense(ctx, req.GroupID, req.Expense)
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
	groupID := r.URL.Query().Get("group_id")
	userID := r.URL.Query().Get("user_id")
	// Sync before getting balances
	s.SyncGroup(ctx, groupID)
	balances := s.GetBalances(groupID, userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balances)
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

	// Start periodic sync
	n.StartPeriodicSync(ctx)

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
