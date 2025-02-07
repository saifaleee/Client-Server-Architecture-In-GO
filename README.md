# Client-Server-Architecture-In-GO

# Distributed Matrix Operations System in Go

A distributed client-server architecture for performing matrix operations using multiple worker nodes, implemented in Go. The system provides fault tolerance, load balancing, and distributed computation capabilities.

## System Architecture

### Components
- **Coordinator (Server)**: Central node that manages task distribution and worker coordination
- **Workers**: Processing nodes that perform matrix computations
- **Client**: Sends computation requests to the coordinator

### Features
- Remote Procedure Calls (RPC) for inter-process communication
- First-Come, First-Served (FCFS) task scheduling
- Load balancing across worker nodes
- Fault tolerance with automatic task reassignment
- Worker health monitoring via heartbeat mechanism

## Supported Matrix Operations
1. Addition
2. Transpose
3. Multiplication

## Getting Started

### Prerequisites
- Go 1.15 or higher
- Network connectivity between client and server machines

### Installation
1. Clone the repository
bash
git clone https://github.com/saifaleee/Client-Server-Architecture-In-GO.git
cd Client-Server-Architecture-In-GO


### Running the System

1. Start the Coordinator:
bash
cd coordinator
go run main.go

2. Start Worker Nodes (run in separate terminals):
bash
cd worker
go run main.go

3. Run the Client:
bash
cd client
go run main.go


## System Design

### Coordinator
- Manages task queue using FCFS scheduling
- Tracks worker status and availability
- Implements load balancing by assigning tasks to least busy workers
- Handles worker failures and task reassignment

### Worker
- Performs matrix computations
- Sends regular heartbeat signals to coordinator
- Handles multiple matrix operations
- Reports computation results back to coordinator

### Client
- Submits computation requests to coordinator
- Receives computed results
- Supports different matrix operations

## Project Structure
.
├── coordinator/
│ └── main.go
├── worker/
│ └── main.go
├── client/
│ └── main.go
└── shared/
└── types.go



## Communication Flow
1. Client submits computation request to coordinator
2. Coordinator adds task to queue
3. Coordinator assigns task to least busy worker
4. Worker performs computation
5. Results are sent back through coordinator to client

## Error Handling
- Worker failure detection through heartbeat mechanism
- Automatic task reassignment on worker failure
- Error reporting back to client

## Future Improvements
- [ ] Add TLS support for secure communication
- [ ] Implement worker node scaling
- [ ] Add support for more matrix operations
- [ ] Implement result caching
- [ ] Add monitoring dashboard

## Contributing
1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License
This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments
- Built as part of distributed systems coursework
- Inspired by real-world distributed computation systems