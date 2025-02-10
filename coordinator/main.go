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

	// Connect to worker's RPC server
	client, err := rpc.Dial("tcp", registration.Address)
	if err != nil {
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

	*reply = true
	return nil
}

// SubmitTask handles incoming client requests
func (s *CoordinatorService) SubmitTask(task *shared.Task, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	// Add task to queue
	s.coord.taskQueue = append(s.coord.taskQueue, *task)

	// Attempt to assign task to least busy worker
	s.assignTasks()

	return nil
}

func (s *CoordinatorService) assignTasks() {
	// Find least busy worker
	var leastBusyWorker *shared.WorkerStatus
	var leastBusyWorkerID string
	minTasks := int(^uint(0) >> 1) // Max int

	for id, worker := range s.coord.workers {
		if worker.Available && worker.TaskCount < minTasks {
			leastBusyWorker = worker
			leastBusyWorkerID = id
			minTasks = worker.TaskCount
		}
	}

	// If we have an available worker and tasks in the queue
	if leastBusyWorker != nil && len(s.coord.taskQueue) > 0 {
		// Get the first task from queue
		task := s.coord.taskQueue[0]
		s.coord.taskQueue = s.coord.taskQueue[1:]

		// Get worker's RPC client
		workerRPC, exists := s.coord.workerRPCs[leastBusyWorkerID]
		if !exists {
			log.Printf("Worker %s not found in RPC map", leastBusyWorkerID)
			return
		}

		// Update worker status
		leastBusyWorker.TaskCount++
		leastBusyWorker.Available = false

		// Send task to worker asynchronously
		go func(task shared.Task, workerRPC *WorkerService, workerStatus *shared.WorkerStatus) {
			var result shared.Result
			err := workerRPC.client.Call("Worker.ProcessTask", task, &result)

			// Lock for updating coordinator state
			s.coord.mu.Lock()
			defer s.coord.mu.Unlock()

			if err != nil {
				log.Printf("Error processing task on worker %s: %v", workerRPC.ID, err)
				// Re-queue the task
				s.coord.taskQueue = append(s.coord.taskQueue, task)
			} else {
				// Store the result
				s.coord.results[task.ID] = result
			}

			// Update worker status
			workerStatus.TaskCount--
			workerStatus.Available = true

			// Try to assign more tasks
			s.assignTasks()
		}(task, workerRPC, leastBusyWorker)
	}
}

// GetResult allows clients to retrieve their computation results
func (s *CoordinatorService) GetResult(taskID string, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	if r, exists := s.coord.results[taskID]; exists {
		*result = r
		delete(s.coord.results, taskID) // Clean up after sending
		return nil
	}

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
		return nil
	}

	return fmt.Errorf("worker %s not found", workerID)
}

// Add a method to check for worker timeouts
func (s *CoordinatorService) checkWorkerTimeouts() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		s.coord.mu.Lock()
		now := time.Now().Unix()

		for id, worker := range s.coord.workers {
			// If no heartbeat received in 15 seconds, mark worker as unavailable
			if now-worker.LastHeartbeat > 15 {
				worker.Available = false
				log.Printf("Worker %s appears to be offline", id)

				// Reassign any tasks from this worker
				if worker.TaskCount > 0 {
					log.Printf("Reassigning tasks from worker %s", id)
					// TODO: Implement task reassignment logic
				}
			}
		}
		s.coord.mu.Unlock()
	}
}

func main() {
	coordinator := NewCoordinator()
	service := &CoordinatorService{coord: coordinator}

	// Start worker timeout checker
	go service.checkWorkerTimeouts()

	server := rpc.NewServer()
	err := server.RegisterName("Coordinator", service)
	if err != nil {
		log.Fatal("Format of service isn't correct:", err)
	}

	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("Listen error:", err)
	}

	log.Println("Coordinator started on :1234")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Accept error:", err)
			continue
		}
		go server.ServeConn(conn)
	}
}
