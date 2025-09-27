# PearBook

![PearBook](https://github.com/Khelechy/pearbook/blob/main/d5c27f66-1f5c-495f-900c-67358ae018a9.jpeg)

A peer-to-peer distributed expense tracker proof-of-concept demonstrating Conflict-Free Replicated Data Types (CRDTs) in a peer-to-peer network using Kademlia DHT, built for research on eventual consistency and the CAP theorem.

[Reference research paper here](https://doi.org/10.64388/IREV9I2-1710338-8995)

## Overview

PearBook is a decentralized application that allows users to track shared expenses without relying on central servers. It uses custom CRDT implementations (OR-Set, PN-Counter, OR-Map) over an actual Kademlia DHT using libp2p to ensure eventual consistency across distributed nodes.

This project serves as a research prototype for exploring:
- CRDTs in distributed systems
- Kademlia DHT for peer-to-peer data storage and retrieval
- Peer-to-peer expense management
- Eventual consistency without central coordination
- CAP theorem trade-offs in real-world applications

## Quick Start

```bash
# Clone and enter the project
git clone https://github.com/khelechy/pearbook.git
cd pearbook/pearbook_core

# Install dependencies
go mod tidy

# Generate cryptographic keys
go run cmd/pearbook/main.go genkey --output my_key.pem

# Start the server
go run cmd/pearbook/main.go server
```

The server will start on `http://localhost:8081` and connect to the libp2p DHT network. Use the CLI tools to generate signed operations for the API endpoints.

## Project Structure

- **pearbook_core/**: Go-based backend implementing the core logic, CRDTs, actual Kademlia DHT using libp2p, HTTP API, and CLI tools. See [README](pearbook_core/README.md) for setup and usage.
  - **CLI Commands**: `genkey` (key generation), `sign` (operation signing), `server` (run API server)
  - **API Endpoints**: RESTful HTTP API with cryptographic security
  - **CRDTs**: OR-Set, OR-Map, PN-Counter implementations
  - **DHT**: Real libp2p Kademlia implementation
- **mobile/** (planned): Mobile app codebase for iOS/Android clients.
- **docs/** (planned): Research papers, CRDT explanations, and architecture docs.

## Features

- **Cryptographic Security**: Client-side ECDSA key generation with digital signatures for all operations
- **CLI Tools**: Command-line interface for key generation (`genkey`), operation signing (`sign`), and server management (`server`)
- **Decentralized Groups**: Create and join expense groups without a central server
- **User Identity Management**: Separation of operational user IDs and display names
- **Single Approval System**: Simplified group joining with single approval instead of majority consensus
- **CRDT-Based Syncing**: Automatic conflict resolution using OR-Set (members), OR-Map (expenses), and PN-Counter (balances)
- **Kademlia DHT Networking**: Actual DHT using libp2p for data storage and retrieval in a real P2P network
- **Global Group Registry**: Decentralized discovery system for finding groups across nodes
- **Cache-First Performance**: Local sharded cache prioritized over network calls for optimal performance
- **RESTful HTTP API**: Proper GET/POST endpoints for group management, expense operations, and balance queries
- **Eventual Consistency**: Merges data from multiple nodes to resolve conflicts
- **Sharded Local Cache**: Hash-based indexing with 16 shards for high concurrency and performance
- **Worker-Based Syncing**: Concurrent worker pools for efficient periodic data propagation

## Architecture

### Core Components
- **CLI Tools**: Command-line interface for key management, operation signing, and server control
- **Cryptographic Security**: ECDSA key pairs generated client-side with digital signature verification
- **Node**: Manages groups with sharded local cache (16 shards), DHT, and CRDT operations using concurrent workers
- **CRDTs**: OR-Set for members, OR-Map for expenses, PN-Counter for balances ensure eventual consistency without conflicts
- **Actual Kademlia DHT using libp2p**: Real P2P network for decentralized data storage and retrieval
- **HTTP API**: RESTful interface with GET/POST methods and JSON request/response formats

### Syncing Mechanism
- **Cache-First**: All operations check local cache before network calls for optimal performance
- **Joining**: Fetches group data when a user joins with automatic local caching
- **Periodic**: Syncs all groups every 5 seconds in the background using concurrent worker pools
- **Global Discovery**: Decentralized group registry enables cross-node group discovery
- **On-Demand**: Syncs before balance queries for up-to-date data with cache updates
- **Merging**: Uses CRDT Merge functions to resolve conflicts and achieve eventual consistency
- **Unique Tags**: Generates UUIDs for each CRDT operation to ensure proper conflict resolution

## CLI Tools

PearBook includes a comprehensive command-line interface for development and testing:

### Key Generation
```bash
go run cmd/pearbook/main.go genkey --output my_key.pem
```
Generates ECDSA key pairs and outputs the public key in hex format ready for API use.

### Operation Signing
```bash
# Create operation data
echo '{"user":{"user_name":"Alice","public_key":"04..."}}' > data.json

# Sign the operation
go run cmd/pearbook/main.go sign --operation join_group --group-id "group123" --user-id "abc123" --data-file data.json --key my_key.pem
```
Creates cryptographically signed operations for secure API interactions.

### Server Management
```bash
go run cmd/pearbook/main.go server
```
Starts the HTTP API server on port 8081 with libp2p DHT connectivity.

## Research Context

This project is designed as a proof-of-concept for academic research on:
- **CRDTs**: Practical implementation of operation-based CRDTs.
- **Kademlia DHT**: Distributed hash table for decentralized data management.
- **CAP Theorem**: Demonstrating eventual consistency in a distributed system.
- **Peer-to-Peer Systems**: Decentralized data management without central authorities.

[Reference research paper here](https://doi.org/10.64388/IREV9I2-1710338-8995)

## Contributing

Contributions are welcome! Please:
1. Fork the repository.
2. Create a feature branch.
3. Submit a pull request with a clear description.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contact

For questions or research collaborations, contact Kelechi Onyekwere at [onyekwerekelechimac@gmail.com].
