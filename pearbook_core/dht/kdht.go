package dht

import (
	"context"
	"fmt"
	"log"
	"time"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func SetupLibp2p(ctx context.Context, bootstrapAddressStr string) (*dht.IpfsDHT, host.Host, error) {
	var bootstrapNode bool
	if len(bootstrapAddressStr) == 0 {
		bootstrapNode = true
	}

	var h host.Host
	var bootstrapPeer *peer.AddrInfo
	var err error

	if bootstrapNode {
		bHost, err := libp2p.New(
			libp2p.ListenAddrStrings(
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/udp/4001/quic",
			),
		)
		if err != nil {
			return nil, nil, err
		}
		h = bHost
	} else {
		bootstrapAddr, err := ma.NewMultiaddr(bootstrapAddressStr)
		if err != nil {
			return nil, nil, fmt.Errorf("Invalid bootstrap address: %w", err)
		}
		cBootstrapPeer, err := peer.AddrInfoFromP2pAddr(bootstrapAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to parse bootstrap peer: %w", err)
		}
		bootstrapPeer = cBootstrapPeer

		cHost, err := libp2p.New(
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), // Random port
		)
		if err != nil {
			log.Fatal(err)
		}

		h = cHost
	}

	defer h.Close()

	// Create datastore
	dstore := dsync.MutexWrap(ds.NewMapDatastore())

	// Create DHT in Server mode and set default peer if its a bootstrap node
	var kadDHT *dht.IpfsDHT
	if bootstrapNode {
		kadDHT, err = dht.New(
			ctx,
			h,
			dht.Datastore(dstore),
			dht.Mode(dht.ModeServer),
			dht.BootstrapPeers(peer.AddrInfo{}),
		)
		if err != nil {
			return nil, nil, err
		}
		if err := kadDHT.Bootstrap(ctx); err != nil {
			log.Printf("Bootstrap error: %v", err)
		}
		
		// Print bootstrap node information
		fmt.Println("🚀 Bootstrap Node Started")
		fmt.Println("========================")
		fmt.Printf("Peer ID: %s\n", h.ID())
		fmt.Println("Listening addresses:")
		for _, addr := range h.Addrs() {
			fmt.Printf("  %s\n", addr)
		}
		fmt.Println("\nBootstrap addresses for clients:")
		for _, addr := range h.Addrs() {
			fullAddr := fmt.Sprintf("%s/p2p/%s", addr, h.ID())
			fmt.Printf("  %s\n", fullAddr)
		}

		// Start periodic status reporting
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					rtSize := kadDHT.RoutingTable().Size()
					connectedPeers := len(h.Network().Peers())
					fmt.Printf("[%s] Routing table: %d peers, Connected: %d peers\n",
						time.Now().Format("15:04:05"), rtSize, connectedPeers)
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		kadDHT, err = dht.New(ctx, h,
			dht.Datastore(dstore),
			dht.Mode(dht.ModeServer),
			dht.BootstrapPeers(*bootstrapPeer),
		)
		if err != nil {
			return nil, nil, err
		}
		fmt.Println("📱 DHT Client Started")
		fmt.Println("====================")
		fmt.Printf("Peer ID: %s\n", h.ID())
		fmt.Printf("Bootstrap peer: %s\n", bootstrapPeer.ID)
		fmt.Println("Listening addresses:")
		for _, addr := range h.Addrs() {
			fmt.Printf("  %s\n", addr)
		}

		// Connect to the bootstrap peer
		fmt.Printf("Connecting to bootstrap peer: %s\n", bootstrapPeer.ID)
		if err := h.Connect(ctx, *bootstrapPeer); err != nil {
			log.Printf("Failed to connect to bootstrap peer: %v", err)
		} else {
			fmt.Println("✓ Connected to bootstrap peer")
		}

		// Bootstrap the DHT
		fmt.Println("Starting DHT bootstrap...")
		if err := kadDHT.Bootstrap(ctx); err != nil {
			log.Printf("Bootstrap error: %v", err)
		} else {
			fmt.Println("✓ DHT bootstrap initiated")
		}

		// Wait for bootstrap to complete
		fmt.Println("Waiting for network discovery...")
		time.Sleep(10 * time.Second)

		// Check routing table size and connected peers
		rtSize := kadDHT.RoutingTable().Size()
		connectedPeers := len(h.Network().Peers())
		fmt.Printf("Routing table size: %d\n", rtSize)
		fmt.Printf("Connected peers: %d\n", connectedPeers)
		fmt.Printf("Connected peer IDs: %v\n", h.Network().Peers())
		if rtSize > 0 {
			fmt.Println("✓ Successfully joined the DHT network!")
		} else {
			fmt.Println("⚠ No peers in routing table - check bootstrap connection")
		}
	}
	return kadDHT, h, nil
}
