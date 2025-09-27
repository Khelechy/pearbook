package commands

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/khelechy/pearbook/pearbook_core/dht"
	"github.com/khelechy/pearbook/pearbook_core/node"
	"github.com/khelechy/pearbook/pearbook_core/server"

	"github.com/common-nighthawk/go-figure"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the PearBook HTTP server",
	Long: `Start the PearBook HTTP server with DHT networking and CRDT synchronization.
The server provides RESTful endpoints for group management, expense tracking,
and cryptographic operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func runServer() {
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
	srv := &server.Server{Node: n}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/createGroup", srv.HandleCreateGroup)
	mux.HandleFunc("/joinGroup", srv.HandleJoinGroup)
	mux.HandleFunc("/addExpense", srv.HandleAddExpense)
	mux.HandleFunc("/getBalances", srv.HandleGetBalances)
	mux.HandleFunc("/getPendingJoins", srv.HandleGetPendingJoins)
	mux.HandleFunc("/approveJoin", srv.HandleApproveJoin)

	srvAddr := ":" + apiPort
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
