package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	"matrix-operations/shared"
)

// WorkerService represents the RPC service exposed by workers
type WorkerService struct {
	ID     string
	client *rpc.Client
}

type Coordinator struct {
	workers    map[string]*shared.WorkerStatus
	workerRPCs map[string]*WorkerService // Add this field
	taskQueue  []shared.Task
	results    map[string]shared.Result
	mu         sync.Mutex
}

type CoordinatorService struct {
	coord *Coordinator
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		workers:    make(map[string]*shared.WorkerStatus),
		workerRPCs: make(map[string]*WorkerService), // Initialize the map
		results:    make(map[string]shared.Result),
	}
}

// RegisterWorker handles worker registration
func (s *CoordinatorService) RegisterWorker(registration shared.WorkerRegistration, reply *bool) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	log.Printf("\n=== Coordinator: Registering new worker ===")
	log.Printf("Worker ID: %s", registration.ID)
	log.Printf("Worker Address: %s", registration.Address)

	// Connect to worker's RPC server
	client, err := rpc.Dial("tcp", registration.Address)
	if err != nil {
		log.Printf("ERROR: Failed to connect to worker %s: %v", registration.ID, err)
		return err
	}

	s.coord.workers[registration.ID] = &shared.WorkerStatus{
		ID:        registration.ID,
		Available: true,
		TaskCount: 0,
	}

	s.coord.workerRPCs[registration.ID] = &WorkerService{
		ID:     registration.ID,
		client: client,
	}

	log.Printf("Worker %s successfully registered", registration.ID)
	log.Printf("Total workers now: %d", len(s.coord.workers))
	*reply = true
	return nil
}

// SubmitTask handles incoming client requests
func (s *CoordinatorService) SubmitTask(task *shared.Task, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	log.Printf("\n=== Coordinator: Received new task ===")
	log.Printf("Task ID: %s", task.ID)
	log.Printf("Operation: %v", task.Operation)
	log.Printf("Matrix 1 dimensions: %dx%d", len(task.Matrix1), len(task.Matrix1[0]))
	if task.Matrix2 != nil {
		log.Printf("Matrix 2 dimensions: %dx%d", len(task.Matrix2), len(task.Matrix2[0]))
	}

	// Add task to queue
	s.coord.taskQueue = append(s.coord.taskQueue, *task)
	log.Printf("Task added to queue. Queue length: %d", len(s.coord.taskQueue))

	// Attempt to assign task to least busy worker
	s.assignTasks()

	return nil
}

func (s *CoordinatorService) assignTasks() {
	log.Printf("\n=== Coordinator: Attempting to assign tasks ===")
	log.Printf("Current queue length: %d", len(s.coord.taskQueue))
	log.Printf("Available workers: %d", len(s.coord.workers))

	// Find least busy worker
	var leastBusyWorker *shared.WorkerStatus
	var leastBusyWorkerID string
	minTasks := int(^uint(0) >> 1) // Max int

	for id, worker := range s.coord.workers {
		log.Printf("Worker %s status - Available: %v, TaskCount: %d", id, worker.Available, worker.TaskCount)
		if worker.Available && worker.TaskCount < minTasks {
			leastBusyWorker = worker
			leastBusyWorkerID = id
			minTasks = worker.TaskCount
		}
	}

	if leastBusyWorker == nil {
		log.Printf("No available workers found")
		return
	}

	// If we have an available worker and tasks in the queue
	if leastBusyWorker != nil && len(s.coord.taskQueue) > 0 {
		log.Printf("Found least busy worker: %s (Current tasks: %d)", leastBusyWorkerID, leastBusyWorker.TaskCount)

		// Get the first task from queue
		task := s.coord.taskQueue[0]
		s.coord.taskQueue = s.coord.taskQueue[1:]
		log.Printf("Assigning task %s to worker %s", task.ID, leastBusyWorkerID)

		// Get worker's RPC client
		workerRPC, exists := s.coord.workerRPCs[leastBusyWorkerID]
		if !exists {
			log.Printf("ERROR: Worker %s not found in RPC map", leastBusyWorkerID)
			return
		}

		// Update worker status
		leastBusyWorker.TaskCount++
		leastBusyWorker.Available = false
		log.Printf("Updated worker %s status - TaskCount: %d, Available: false", leastBusyWorkerID, leastBusyWorker.TaskCount)

		// Send task to worker asynchronously
		go func(task shared.Task, workerRPC *WorkerService, workerStatus *shared.WorkerStatus) {
			log.Printf("Starting async task processing for task %s on worker %s", task.ID, workerRPC.ID)
			var result shared.Result
			err := workerRPC.client.Call("Worker.ProcessTask", task, &result)

			// Lock for updating coordinator state
			s.coord.mu.Lock()
			defer s.coord.mu.Unlock()

			if err != nil {
				log.Printf("ERROR: Processing task %s on worker %s failed: %v", task.ID, workerRPC.ID, err)
				// Re-queue the task
				s.coord.taskQueue = append(s.coord.taskQueue, task)
				log.Printf("Task %s re-queued. New queue length: %d", task.ID, len(s.coord.taskQueue))
			} else {
				log.Printf("Task %s completed successfully by worker %s", task.ID, workerRPC.ID)
				// Store the result
				s.coord.results[task.ID] = result
			}

			// Update worker status
			workerStatus.TaskCount--
			workerStatus.Available = true
			log.Printf("Updated worker %s status - TaskCount: %d, Available: true", workerRPC.ID, workerStatus.TaskCount)

			// Try to assign more tasks
			s.assignTasks()
		}(task, workerRPC, leastBusyWorker)
	}
}

// GetResult allows clients to retrieve their computation results
func (s *CoordinatorService) GetResult(taskID string, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	log.Printf("\n=== Coordinator: Result request ===")
	log.Printf("Requested Task ID: %s", taskID)

	if r, exists := s.coord.results[taskID]; exists {
		log.Printf("Result found for task %s", taskID)
		*result = r
		delete(s.coord.results, taskID) // Clean up after sending
		return nil
	}

	log.Printf("Result not found for task %s", taskID)
	return fmt.Errorf("result not found for task %s", taskID)
}

// Heartbeat handles worker heartbeat signals
func (s *CoordinatorService) Heartbeat(workerID string, reply *bool) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	if worker, exists := s.coord.workers[workerID]; exists {
		worker.LastHeartbeat = time.Now().Unix()
		worker.Available = true
		*reply = true
		log.Printf("Received heartbeat from worker %s", workerID)
		return nil
	}

	log.Printf("ERROR: Heartbeat received from unknown worker %s", workerID)
	return fmt.Errorf("worker %s not found", workerID)
}

// Add a method to check for worker timeouts
func (s *CoordinatorService) checkWorkerTimeouts() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		s.coord.mu.Lock()
		now := time.Now().Unix()
		log.Printf("\n=== Coordinator: Checking worker timeouts ===")

		for id, worker := range s.coord.workers {
			timeSinceLastHeartbeat := now - worker.LastHeartbeat
			log.Printf("Worker %s - Time since last heartbeat: %d seconds", id, timeSinceLastHeartbeat)

			// If no heartbeat received in 15 seconds, mark worker as unavailable
			if timeSinceLastHeartbeat > 15 {
				worker.Available = false
				log.Printf("WARNING: Worker %s appears to be offline", id)

				// Reassign any tasks from this worker
				if worker.TaskCount > 0 {
					log.Printf("WARNING: Worker %s has %d unfinished tasks - will be reassigned", id, worker.TaskCount)
					// TODO: Implement task reassignment logic
				}
			}
		}
		s.coord.mu.Unlock()
	}
}

func main() {
	log.Printf("=== Coordinator Starting ===")
	coordinator := NewCoordinator()
	service := &CoordinatorService{coord: coordinator}

	log.Printf("Starting worker timeout checker...")
	go service.checkWorkerTimeouts()

	server := rpc.NewServer()
	err := server.RegisterName("Coordinator", service)
	if err != nil {
		log.Fatal("ERROR: Failed to register RPC server:", err)
	}

	// Find the best IP address to use
	var bestIP string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipv4 := ipnet.IP.To4(); ipv4 != nil {
					log.Printf("Found network interface: %s", ipv4.String())
					// Prefer 192.168.x.x addresses
					if ipv4[0] == 192 && ipv4[1] == 168 {
						bestIP = ipv4.String()
						break
					}
					// Otherwise use the first non-loopback IPv4 address
					if bestIP == "" {
						bestIP = ipv4.String()
					}
				}
			}
		}
	}

	if bestIP == "" {
		log.Fatal("ERROR: No suitable network interface found")
	}

	log.Printf("Using IP address: %s", bestIP)

	// Listen on all interfaces
	listener, err := net.Listen("tcp", "0.0.0.0:1234")
	if err != nil {
		log.Fatal("ERROR: Listen error:", err)
	}

	log.Printf("Coordinator listening on port 1234")
	log.Printf("Ready to accept connections")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("ERROR: Accept error: %v", err)
			continue
		}
		log.Printf("New connection from: %s", conn.RemoteAddr().String())
		go server.ServeConn(conn)
	}
}
