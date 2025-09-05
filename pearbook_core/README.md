# PearBook: Distributed Expense Tracker

A proof-of-concept implementation of a peer-to-peer expense sharing application using Conflict-free Replicated Data Types (CRDTs) over a simulated Kademlia Distributed Hash Table (DHT). This project demonstrates eventual consistency in distributed systems, aligning with the CAP theorem's trade-offs for high availability and partition tolerance without a central server.

## Project Overview

PearBook allows users to create groups, join them, add expenses, and track balances in a decentralized manner. It uses CRDTs to ensure data consistency across replicas, even in network partitions. This is designed as a research tool for exploring distributed systems concepts, particularly for academic papers on CRDTs and DHTs.

### Key Technologies
- **Language**: Go 1.19+
- **CRDTs**: Custom implementations of OR-Set (for group members), OR-Map (for expenses), and PN-Counter (for balances)
- **DHT**: Simulated Kademlia DHT (in-memory for PoC; can be replaced with libp2p)
- **Networking**: HTTP API for client interactions
- **Storage**: In-memory (simulated DHT)

## Features
- **Decentralized Groups**: Create and join groups without a central authority.
- **Expense Management**: Add expenses with equal splits, track participants and payers (using OR-Map CRDT for conflict-free storage).
- **Balance Tracking**: CRDT-based balances for owed amounts, ensuring conflict-free updates (using PN-Counter CRDT).
- **Offline Support**: Local replicas with eventual sync via DHT (periodic syncing every 5 seconds and on-demand for reads).
- **HTTP API**: RESTful endpoints for client applications.
- **Testing**: Unit tests for core functionality.

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

1. **Start the Server**:
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

## API Usage

The application exposes a RESTful HTTP API for client interactions. All requests/responses use JSON.

### Endpoints

#### 1. Create a Group
- **Endpoint**: `POST /createGroup`
- **Request Body**:
  ```json
  {
    "groupId": "group1",
    "name": "Trip to Paris",
    "creator": "alice"
  }
  ```
- **Response**: `200 OK` with "Group created"
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/createGroup \
    -H "Content-Type: application/json" \
    -d '{"groupId": "group1", "name": "Trip", "creator": "alice"}'
  ```

#### 2. Join a Group
- **Endpoint**: `POST /joinGroup`
- **Request Body**:
  ```json
  {
    "groupId": "group1",
    "userId": "bob"
  }
  ```
- **Response**: `200 OK` with "Joined group"
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/joinGroup \
    -H "Content-Type: application/json" \
    -d '{"groupId": "group1", "userId": "bob"}'
  ```

#### 3. Add an Expense
- **Endpoint**: `POST /addExpense`
- **Request Body**:
  ```json
  {
    "groupId": "group1",
    "expense": {
      "id": "exp1",
      "amount": 100.0,
      "description": "Dinner",
      "payer": "alice",
      "participants": ["alice", "bob"]
    }
  }
  ```
- **Response**: `200 OK` with "Expense added"
- **Notes**: Splits are calculated equally; balances updated using PN-Counter CRDT.
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/addExpense \
    -H "Content-Type: application/json" \
    -d '{"groupId": "group1", "expense": {"id": "exp1", "amount": 100.0, "description": "Dinner", "payer": "alice", "participants": ["alice", "bob"]}}'
  ```

#### 4. Get Balances
- **Endpoint**: `GET /getBalances?groupId=<groupId>&userId=<userId>`
- **Response**: `200 OK` with JSON like `{"alice": 50.0}`
- **Notes**: Syncs with DHT before returning balances for freshness.
- **Example**:
  ```bash
  curl "http://localhost:8080/getBalances?groupId=group1&userId=bob"
  ```

### Error Handling
- Invalid requests return `400 Bad Request` or `500 Internal Server Error` with error messages.
- Ensure `groupId` and `userId` are valid strings.

## Testing

### Unit Tests
Run the built-in tests:
```bash
go test
```
This executes tests for group creation, joining, expense addition, and balance retrieval. Expected output: `PASS` with test counts.

### Manual Testing
1. Start the server as described.
2. Use curl commands above to interact via API.
3. Verify balances update correctly after adding expenses.
4. Test edge cases: Non-existent groups, invalid users, etc.

### Integration Testing
- Simulate multiple nodes by running multiple instances (modify ports).
- Use the simulated DHT to test data replication (though in-memory, it's local).

## Architecture

### Directory Structure
```
pearbook_core/
├── main.go          # Main application and HTTP handlers
├── main_test.go     # Unit tests
├── models/          # Data structures (User, Group, Expense)
├── crdt/            # CRDT implementations (OR-Set, PN-Counter)
├── dht/             # Simulated DHT
├── go.mod           # Go module file
└── README.md        # This file
```

### Core Components
- **Node**: Manages groups, DHT, and CRDT operations.
- **CRDTs**: OR-Set for members, OR-Map for expenses, PN-Counter for balances—ensure eventual consistency without conflicts.
- **Simulated DHT**: In-memory storage for PoC; replace with libp2p for real P2P.
- **HTTP API**: Simple REST interface for clients.

### Design Principles
- **Decentralized**: No central server; data replicated via DHT.
- **Eventual Consistency**: CRDTs handle concurrent updates.
- **Partition Tolerant**: Works offline with local replicas.
- **High Availability**: DHT ensures data accessibility.

## Research Context
This implementation serves as a case study for:
- **CAP Theorem**: Prioritizing Availability and Partition Tolerance over strict Consistency.
- **CRDTs in Practice**: Demonstrating OR-Set for sets and PN-Counter for counters.
- **DHT Scalability**: Simulating Kademlia for distributed storage.

For your paper, reference the code for real-world CRDT applications in expense tracking.

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

**Note**: This is a proof-of-concept. For production, integrate real DHT (e.g., libp2p), add authentication, and handle larger datasets.