package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	"matrix-operations/shared"
)

type WorkerService struct {
	ID     string
	client *rpc.Client
}

type Coordinator struct {
	workers    map[string]*shared.WorkerStatus
	workerRPCs map[string]*WorkerService
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
		workerRPCs: make(map[string]*WorkerService),
		results:    make(map[string]shared.Result),
	}
}

func (s *CoordinatorService) RegisterWorker(registration shared.WorkerRegistration, reply *bool) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	client, err := rpc.Dial("tcp", registration.Address)
	if err != nil {
		return err
	}

	s.coord.workers[registration.ID] = &shared.WorkerStatus{
		ID:            registration.ID,
		Available:     true,
		TaskCount:     0,
		LastHeartbeat: time.Now().Unix(),
	}

	s.coord.workerRPCs[registration.ID] = &WorkerService{
		ID:     registration.ID,
		client: client,
	}

	*reply = true
	log.Printf("Worker %s registered at %s", registration.ID, registration.Address)
	return nil
}

func (s *CoordinatorService) SubmitTask(task *shared.Task, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	s.coord.taskQueue = append(s.coord.taskQueue, *task)
	log.Printf("Task %s added to queue", task.ID)

	s.assignTasks()
	return nil
}

func (s *CoordinatorService) assignTasks() {
	for len(s.coord.taskQueue) > 0 {
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

		if leastBusyWorker == nil {
			log.Println("No available workers to assign tasks.")
			return
		}

		task := s.coord.taskQueue[0]
		s.coord.taskQueue = s.coord.taskQueue[1:]

		workerRPC := s.coord.workerRPCs[leastBusyWorkerID]
		leastBusyWorker.TaskCount++
		leastBusyWorker.Available = false

		go s.processTask(task, workerRPC, leastBusyWorker)
	}
}

func (s *CoordinatorService) processTask(task shared.Task, workerRPC *WorkerService, workerStatus *shared.WorkerStatus) {
	var result shared.Result
	err := workerRPC.client.Call("Worker.ProcessTask", task, &result)

	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	if err != nil {
		log.Printf("Error processing task %s on worker %s: %v", task.ID, workerRPC.ID, err)
		s.coord.taskQueue = append(s.coord.taskQueue, task)
	} else {
		s.coord.results[task.ID] = result
		log.Printf("Task %s completed by worker %s", task.ID, workerRPC.ID)
	}

	workerStatus.TaskCount--
	workerStatus.Available = true
	s.assignTasks()
}

func (s *CoordinatorService) GetResult(taskID string, result *shared.Result) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	if r, exists := s.coord.results[taskID]; exists {
		*result = r
		delete(s.coord.results, taskID)
		return nil
	}

	return fmt.Errorf("result not found for task %s", taskID)
}

func (s *CoordinatorService) Heartbeat(workerID string, reply *bool) error {
	s.coord.mu.Lock()
	defer s.coord.mu.Unlock()

	if worker, exists := s.coord.workers[workerID]; exists {
		worker.LastHeartbeat = time.Now().Unix()
		worker.Available = worker.TaskCount == 0 // Only mark as available if not processing tasks
		*reply = true
		return nil
	}

	return fmt.Errorf("worker %s not found", workerID)
}

func (s *CoordinatorService) checkWorkerTimeouts() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		s.coord.mu.Lock()
		now := time.Now().Unix()

		for id, worker := range s.coord.workers {
			if now-worker.LastHeartbeat > 15 {
				worker.Available = false
				log.Printf("Worker %s appears offline", id)
			}
		}
		s.coord.mu.Unlock()
	}
}

func main() {
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatalf("Failed to load server certificate and key: %v", err)
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", ":1234", config)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	defer listener.Close()

	coordinator := NewCoordinator()
	service := &CoordinatorService{coord: coordinator}
	rpc.Register(service)

	go service.checkWorkerTimeouts()

	log.Println("Coordinator service running on :1234")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
