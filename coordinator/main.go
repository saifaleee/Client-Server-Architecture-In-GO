package main

import (
	"log"
	"net"
	"net/rpc"
	"sync"

	"../shared"
)

type Coordinator struct {
	workers   map[string]*shared.WorkerStatus
	taskQueue []shared.Task
	results   map[string]shared.Result
	mu        sync.Mutex
}

type CoordinatorService struct {
	coord *Coordinator
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		workers: make(map[string]*shared.WorkerStatus),
		results: make(map[string]shared.Result),
	}
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
	minTasks := int(^uint(0) >> 1) // Max int

	for _, worker := range s.coord.workers {
		if worker.Available && worker.TaskCount < minTasks {
			leastBusyWorker = worker
			minTasks = worker.TaskCount
		}
	}

	if leastBusyWorker != nil && len(s.coord.taskQueue) > 0 {
		// Assign task to worker
		task := s.coord.taskQueue[0]
		s.coord.taskQueue = s.coord.taskQueue[1:]
		leastBusyWorker.TaskCount++

		// TODO: Send task to worker

	}
}

func main() {
	coordinator := NewCoordinator()
	service := &CoordinatorService{coord: coordinator}

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
