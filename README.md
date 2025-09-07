# PearBook

An offline first distributed expense tracker proof-of-concept demonstrating Conflict-Free Replicated Data Types (CRDTs) in a peer-to-peer network using Kademlia DHT, built for research on eventual consistency and the CAP theorem.

## Overview

PearBook is a decentralized application that allows users to track shared expenses without relying on central servers. It uses custom CRDT implementations (OR-Set, PN-Counter, OR-Map) over an actual Kademlia DHT using libp2p to ensure eventual consistency across distributed nodes.

This project serves as a research prototype for exploring:
- CRDTs in distributed systems
- Kademlia DHT for peer-to-peer data storage and retrieval
- Peer-to-peer expense management
- Eventual consistency without central coordination
- CAP theorem trade-offs in real-world applications

## Project Structure

- **pearbook_core/**: Go-based backend implementing the core logic, CRDTs, actual Kademlia DHT using libp2p, and HTTP API. See [README](pearbook_core/README.md) for setup and usage.
- **mobile/** (planned): Mobile app codebase for iOS/Android clients.
- **docs/** (planned): Research papers, CRDT explanations, and architecture docs.

## Features

- **Decentralized Groups**: Create and join expense groups without a central server.
- **CRDT-Based Syncing**: Automatic conflict resolution using OR-Set (members), OR-Map (expenses), and PN-Counter (balances).
- **Kademlia DHT Networking**: Actual DHT using libp2p for data storage and retrieval in a real P2P network.
- **HTTP API**: RESTful endpoints for group management, expense addition, and balance queries.
- **Eventual Consistency**: Merges data from multiple nodes to resolve conflicts.

## Architecture

### Core Components
- **Node**: Manages groups, DHT, and CRDT operations.
- **CRDTs**: OR-Set for members, OR-Map for expenses, PN-Counter for balancesensure eventual consistency without conflicts.
- **Actual Kademlia DHT using libp2p**: Real P2P network for decentralized data storage and retrieval.
- **HTTP API**: Simple REST interface for clients.

### Syncing Mechanism
- **Joining**: Fetches group data when a user joins.
- **Periodic**: Syncs all groups every 5 seconds in the background.
- **On-Demand**: Syncs before balance queries for up-to-date data.
- **Merging**: Uses CRDT Merge functions to resolve conflicts and achieve eventual consistency.
- **Unique Tags**: Generates UUIDs for each CRDT operation to ensure proper conflict resolution.

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
