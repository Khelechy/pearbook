# PearBook: Distributed Expense Tracker

A proof-of-concept implementation of a peer-to-peer expense sharing application using Conflict-free Replicated Data Types (CRDTs) over an actual Kademlia Distributed Hash Table (DHT) using libp2p. This project demonstrates eventual consistency in distributed systems, aligning with the CAP theorem's trade-offs for high availability and partition tolerance without a central server.

## Project Overview

PearBook allows users to create groups, join them, add expenses, and track balances in a decentralized manner. It uses CRDTs to ensure data consistency across replicas, even in network partitions. This is designed as a research tool for exploring distributed systems concepts, particularly for academic papers on CRDTs and DHTs.

### Key Technologies
- **Language**: Go 1.19+
- **Cryptography**: ECDSA digital signatures with client-side key generation
- **CRDTs**: Custom implementations of OR-Set (for group members), OR-Map (for expenses), and PN-Counter (for balances)
- **DHT**: Actual Kademlia DHT using libp2p
- **Networking**: RESTful HTTP API with GET/POST methods
- **Storage**: Distributed via libp2p DHT
- **User Identity**: Separation of operational user IDs (public key derived) and display names
- **Sharding**: 16-shard hash-based local cache for concurrency
- **Concurrency**: Worker pools for efficient syncing operations

## Features
- **Cryptographic Security**: Client-side ECDSA key generation with digital signature verification for all write operations
- **User Identity Management**: Separation of operational user IDs (derived from public keys) and display names
- **Decentralized Groups**: Create and join groups with approval-based membership
- **Expense Management**: Add expenses with equal splits, track participants and payers (using OR-Map CRDT for conflict-free storage)
- **Balance Tracking**: CRDT-based balances for owed amounts with enriched responses including user names
- **Offline Support**: Local replicas with eventual sync via DHT (periodic syncing every 5 seconds and on-demand for reads)
- **RESTful HTTP API**: GET/POST endpoints with JSON request/response formats
- **Testing**: Unit tests for core functionality including cryptographic operations
- **Performance Optimizations**: Sharded cache and concurrent worker syncing for high throughput

## Prerequisites
- Go 1.19 or later (download from [golang.org](https://golang.org/dl/))
- Git (for cloning the repository)
- A terminal/command prompt
- Optional: Postman or curl for API testing

## Installation and Setup

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/khelechy/pearbook.git
   cd pearbook/pearbook_core
   ```

2. **Install Dependencies**:
   The project uses Go modules. Run:
   ```bash
   go mod tidy
   ```
   This will download all required dependencies.

3. **Build the Application**:
   ```bash
   go build
   ```
   This creates an executable `pearbook_core.exe` (on Windows) or `pearbook_core` (on Linux/Mac).

## Running the Application

   ```bash
   go run main.go
   ```
   Or, after building:
   ```bash
   ./pearbook_core
   ```

   The server starts on `http://localhost:8080` and performs a demo setup (creates a group, adds a user, and an expense).

2. **Server Output**:
   - Console will show: "Starting server on :8080"
   - Demo balances: "Bob's balances: map[alice:50]"

3. **Stop the Server**:
   Press `Ctrl+C` in the terminal.

  The server now supports graceful shutdown: when you press `Ctrl+C` (SIGINT) or send a SIGTERM signal, it will finish handling any in-progress requests before exiting cleanly.

## API Usage

The application exposes a RESTful HTTP API for client interactions. All requests/responses use JSON. The API implements cryptographic security with client-side key generation and digital signatures.

### Prerequisites for API Usage

**Client-Side Key Generation:**
Before using the API, clients must generate ECDSA key pairs:

```go
// Example Go code for key generation
privateKey, publicKey, err := utils.GenerateKeyPair()
userID := utils.GenerateUserID(publicKey)
```

**Digital Signatures:**
For write operations, clients must sign operation data:

```go
// Create operation data
opData := utils.CreateOperationData(operation, groupID, userID, timestamp, data)

// Sign with private key
signature, err := utils.SignData(privateKey, opData)
```

### Endpoints

#### 1. Create a Group
- **Endpoint**: `POST /createGroup`
- **Description**: Creates a new expense group with the creator as the first approved member
- **Request Body**:
  ```json
  {
    "operation": "create_group",
    "group_id": "group123",
    "user_id": "a1b2c3d4e5f67890",
    "user_name": "Alice Johnson",
    "timestamp": 1638360000,
    "data": {
      "group": {
        "id": "group123",
        "name": "Trip to Paris"
      },
      "user": {
        "user_name": "Alice Johnson",
        "public_key": "0462c2d3...[65-byte hex-encoded public key]"
      }
    },
    "signature": "1a2b3c4d...[64-byte hex-encoded signature]"
  }
  ```
- **Response**: `200 OK` with `"Group created"`
- **Security**: Requires valid digital signature from creator

#### 2. Join a Group
- **Endpoint**: `POST /joinGroup`
- **Description**: Submits a join request for an existing group (requires approval from existing members)
- **Request Body**:
  ```json
  {
    "operation": "join_group",
    "group_id": "group123",
    "user_id": "b2c3d4e5f6789012",
    "user_name": "Bob Smith",
    "timestamp": 1638360001,
    "data": {
      "user": {
        "user_name": "Bob Smith",
        "public_key": "0462c2d3...[65-byte hex-encoded public key]"
      }
    },
    "signature": "2b3c4d5e...[64-byte hex-encoded signature]"
  }
  ```
- **Response**: `200 OK` with `"Join request submitted"`
- **Security**: Requires valid digital signature from requester

#### 3. Approve Join Request
- **Endpoint**: `POST /approveJoin`
- **Description**: Approves a pending join request (requires majority approval from existing members)
- **Request Body**:
  ```json
  {
    "operation": "approve_join",
    "group_id": "group123",
    "user_id": "a1b2c3d4e5f67890",
    "timestamp": 1638360002,
    "data": {
      "request_id": "group123:b2c3d4e5f6789012"
    },
    "signature": "3c4d5e6f...[64-byte hex-encoded signature]"
  }
  ```
- **Response**: `200 OK` with `"Join request approved"`
- **Security**: Requires valid digital signature from approver

#### 4. Add an Expense
- **Endpoint**: `POST /addExpense`
- **Description**: Adds a new expense to a group with equal splits among participants
- **Request Body**:
  ```json
  {
    "operation": "add_expense",
    "group_id": "group123",
    "user_id": "a1b2c3d4e5f67890",
    "timestamp": 1638360003,
    "data": {
      "expense": {
        "id": "exp1",
        "amount": 100.0,
        "description": "Dinner at restaurant",
        "participants": ["a1b2c3d4e5f67890", "b2c3d4e5f6789012"]
      }
    },
    "signature": "4d5e6f7g...[64-byte hex-encoded signature]"
  }
  ```
- **Response**: `200 OK` with `"Expense added"`
- **Notes**: Splits are calculated equally; balances updated using PN-Counter CRDT
- **Security**: Requires valid digital signature from expense creator

#### 5. Get Balances
- **Endpoint**: `GET /getBalances?group_id=group123&user_id=a1b2c3d4e5f67890`
- **Description**: Retrieves balance information for a user in a group
- **Response**: `200 OK` with enriched balance data:
  ```json
  {
    "a1b2c3d4e5f67890": {
      "user_id": "a1b2c3d4e5f67890",
      "user_name": "Alice Johnson",
      "amount": 50.0
    }
  }
  ```
- **Notes**: Returns enriched data with both user IDs and display names

#### 6. Get Pending Joins
- **Endpoint**: `GET /getPendingJoins?group_id=group123&user_id=a1b2c3d4e5f67890`
- **Description**: Retrieves all pending join requests for a group
- **Response**: `200 OK` with array of pending requests:
  ```json
  {
    "group123:b2c3d4e5f6789012": {
      "requester_id": "b2c3d4e5f6789012",
      "user_name": "Bob Smith",
      "public_key": "0462c2d3...",
      "timestamp": 1638360001,
      "approvals": {
        "a1b2c3d4e5f67890": "signature_data..."
      }
    }
  }
  ```
- **Notes**: Only accessible by approved group members

### Error Handling
- **400 Bad Request**: Invalid JSON, missing required fields, or malformed data
- **401 Unauthorized**: Invalid digital signature or insufficient permissions
- **404 Not Found**: Group or user not found
- **500 Internal Server Error**: Server-side errors with descriptive messages

### Security Model
- **Write Operations**: Require valid ECDSA digital signatures
- **Read Operations**: Require only group membership verification
- **Key Management**: Public keys are stored with user profiles for signature verification
- **User IDs**: Derived from public keys using SHA256 hash for uniqueness
- **Display Names**: User-provided names stored separately from operational IDs

## Testing

### Unit Tests
Run the built-in tests:
```bash
go test
```
This executes tests for group creation, joining, expense addition, balance retrieval, and cryptographic operations. Expected output: `PASS` with test counts.

### Manual Testing with Cryptographic Operations

#### Generate Test Keys and User IDs
```go
package main

import (
    "fmt"
    "github.com/khelechy/pearbook/utils"
)

func main() {
    // Generate keys for Alice
    alicePrivate, alicePublic, _ := utils.GenerateKeyPair()
    aliceUserID := utils.GenerateUserID(alicePublic)
    fmt.Printf("Alice User ID: %s\n", aliceUserID)
    fmt.Printf("Alice Public Key: %x\n", alicePublic)

    // Generate keys for Bob
    bobPrivate, bobPublic, _ := utils.GenerateKeyPair()
    bobUserID := utils.GenerateUserID(bobPublic)
    fmt.Printf("Bob User ID: %s\n", bobUserID)
    fmt.Printf("Bob Public Key: %x\n", bobPublic)

    // Example: Sign operation data
    opData := utils.CreateOperationData("create_group", "group123", aliceUserID, 1638360000, map[string]interface{}{
        "group": map[string]interface{}{
            "id": "group123",
            "name": "Test Group",
        },
        "user": map[string]interface{}{
            "user_name": "Alice Johnson",
            "public_key": fmt.Sprintf("%x", alicePublic),
        },
    })

    signature, _ := utils.SignData(alicePrivate, opData)
    fmt.Printf("Signature: %x\n", signature)
}
```

#### Test API Endpoints
Use the generated keys and signatures in API requests:

```bash
# Create group with cryptographic signature
curl -X POST http://localhost:8080/createGroup \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "create_group",
    "group_id": "group123",
    "user_id": "'$ALICE_USER_ID'",
    "user_name": "Alice Johnson",
    "timestamp": 1638360000,
    "data": {
      "group": {"id": "group123", "name": "Test Group"},
      "user": {"user_name": "Alice Johnson", "public_key": "'$ALICE_PUBLIC_KEY'"}
    },
    "signature": "'$SIGNATURE'"
  }'

# Get balances (no signature required)
curl "http://localhost:8080/getBalances?group_id=group123&user_id=$ALICE_USER_ID"
```

### Integration Testing
- Simulate multiple nodes by running multiple instances (modify ports)
- Use the actual libp2p DHT to test data replication across peers
- Test cryptographic signature verification with invalid signatures

## Architecture

### Directory Structure
```
pearbook_core/
├── main.go          # Main application and HTTP handlers
├── main_test.go     # Unit tests
├── models/          # Data structures (User, Group, Expense)
├── crdt/            # CRDT implementations (OR-Set, PN-Counter)
├── dht/             # Actual Kademlia DHT using libp2p
├── go.mod           # Go module file
└── README.md        # This file
```

### Core Components
- **Cryptographic Security**: ECDSA key pairs generated client-side with digital signature verification for write operations
- **User Identity Management**: Separation of operational user IDs (derived from public keys) and display names
- **Node**: Manages groups with a sharded local cache (16 shards for concurrency), DHT interactions, and CRDT operations using worker pools for syncing
- **CRDTs**: OR-Set for members, OR-Map for expenses, PN-Counter for balances—ensure eventual consistency without conflicts
- **Actual Kademlia DHT using libp2p**: Real P2P network for decentralized data storage and retrieval
- **RESTful HTTP API**: Proper GET/POST endpoints with JSON request/response formats and cryptographic security

### Syncing Mechanism
- **Joining**: Fetches group data when a user joins.
- **Periodic**: Syncs all groups every 5 seconds in the background using concurrent worker pools for efficiency.
- **On-Demand**: Syncs before balance queries for up-to-date data.
- **Merging**: Uses CRDT Merge functions to resolve conflicts and achieve eventual consistency.
- **Unique Tags**: Generates UUIDs for each CRDT operation to ensure proper conflict resolution.

### Design Principles
- **Decentralized**: No central server; data replicated via DHT.
- **Eventual Consistency**: CRDTs handle concurrent updates.
- **Partition Tolerant**: Works offline with local replicas.
- **High Availability**: DHT ensures data accessibility.

## Performance Optimizations

- **Sharding**: Local cache divided into 16 shards using hash-based indexing to reduce lock contention and improve concurrency.
- **Worker Pools**: Concurrent workers for periodic syncing, allowing multiple groups to sync simultaneously without blocking.
- **Lazy Balance Computation**: Balances calculated on-demand to minimize unnecessary computations.
- **Cache Invalidation**: Efficient invalidation on updates to ensure data freshness.

## Research Context
This implementation serves as a case study for:
- **Cryptographic Security in P2P Systems**: Client-side key generation and digital signature verification
- **User Identity Management**: Separation of operational identifiers and display names
- **CAP Theorem**: Prioritizing Availability and Partition Tolerance over strict Consistency
- **CRDTs in Practice**: Demonstrating OR-Set for sets and PN-Counter for counters with security
- **DHT Scalability**: Using actual Kademlia DHT with libp2p for distributed storage
- **Digital Signatures**: ECDSA implementation for operation authentication in decentralized systems

[Reference research paper here](https://doi.org/10.64388/IREV9I2-1710338-8995)

## Contributing
1. Fork the repository.
2. Create a feature branch.
3. Make changes and add tests.
4. Submit a pull request.

## License
This project is for educational/research purposes. No specific license applied.

## Support
For issues or questions, open a GitHub issue or contact the maintainer.

---

**Note**: This is a proof-of-concept demonstrating cryptographic security and CRDTs in distributed systems. For production use, consider additional security measures, handle larger datasets, optimize the libp2p DHT configuration, and implement proper key management and rotation strategies.
