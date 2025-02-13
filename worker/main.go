package main

import (
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"matrix-operations/shared"
)

type Worker struct {
	ID              string
	coordinatorAddr string
}

type WorkerRPC struct {
	worker *Worker
}

func NewWorker(id string, coordinatorAddr string) *Worker {
	return &Worker{
		ID:              id,
		coordinatorAddr: coordinatorAddr,
	}
}

// Matrix operations
func (w *Worker) Add(m1, m2 [][]float64) ([][]float64, error) {
	log.Printf("Worker %s: Starting Addition operation", w.ID)
	rows, cols := len(m1), len(m1[0])
	result := make([][]float64, rows)

	for i := range result {
		result[i] = make([]float64, cols)
		for j := range result[i] {
			result[i][j] = m1[i][j] + m2[i][j]
		}
	}
	log.Printf("Worker %s: Addition completed", w.ID)
	return result, nil
}

func (w *Worker) Transpose(m [][]float64) ([][]float64, error) {
	log.Printf("Worker %s: Starting Transpose operation", w.ID)
	rows, cols := len(m), len(m[0])
	result := make([][]float64, cols)

	for i := range result {
		result[i] = make([]float64, rows)
		for j := range result[i] {
			result[i][j] = m[j][i]
		}
	}
	log.Printf("Worker %s: Transpose completed", w.ID)
	return result, nil
}

func (w *Worker) Multiply(m1, m2 [][]float64) ([][]float64, error) {
	log.Printf("Worker %s: Starting Multiplication operation", w.ID)
	rows1, cols1 := len(m1), len(m1[0])
	cols2 := len(m2[0])
	result := make([][]float64, rows1)

	for i := range result {
		result[i] = make([]float64, cols2)
		for j := 0; j < cols2; j++ {
			sum := 0.0
			for k := 0; k < cols1; k++ {
				sum += m1[i][k] * m2[k][j]
			}
			result[i][j] = sum
		}
	}
	log.Printf("Worker %s: Multiplication completed", w.ID)
	return result, nil
}

func (w *Worker) ProcessTask(task shared.Task) shared.Result {
	log.Printf("\n=== Worker %s: Received new task ===", w.ID)
	log.Printf("Task ID: %s", task.ID)
	log.Printf("Operation: %v", task.Operation)
	log.Printf("Matrix 1 dimensions: %dx%d", len(task.Matrix1), len(task.Matrix1[0]))
	if task.Matrix2 != nil {
		log.Printf("Matrix 2 dimensions: %dx%d", len(task.Matrix2), len(task.Matrix2[0]))
	}

	var result shared.Result
	result.TaskID = task.ID

	var err error
	switch task.Operation {
	case shared.Addition:
		log.Printf("Worker %s: Processing Addition task", w.ID)
		result.Matrix, err = w.Add(task.Matrix1, task.Matrix2)
	case shared.Transpose:
		log.Printf("Worker %s: Processing Transpose task", w.ID)
		result.Matrix, err = w.Transpose(task.Matrix1)
	case shared.Multiplication:
		log.Printf("Worker %s: Processing Multiplication task", w.ID)
		result.Matrix, err = w.Multiply(task.Matrix1, task.Matrix2)
	}

	if err != nil {
		log.Printf("Worker %s: Error processing task: %v", w.ID, err)
		result.Error = err.Error()
	} else {
		log.Printf("Worker %s: Task completed successfully", w.ID)
		log.Printf("Result matrix dimensions: %dx%d", len(result.Matrix), len(result.Matrix[0]))
	}

	log.Printf("=== Worker %s: Task processing complete ===\n", w.ID)
	return result
}

func (w *Worker) startHeartbeat() {
	log.Printf("Worker %s: Starting heartbeat service", w.ID)
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		client, err := rpc.Dial("tcp", w.coordinatorAddr)
		if err != nil {
			log.Printf("Worker %s: Failed to connect to coordinator: %v", w.ID, err)
			continue
		}

		var reply bool
		err = client.Call("Coordinator.Heartbeat", w.ID, &reply)
		if err != nil {
			log.Printf("Worker %s: Heartbeat failed: %v", w.ID, err)
		} else {
			log.Printf("Worker %s: Heartbeat sent successfully", w.ID)
		}
		client.Close()
	}
}

func (w *WorkerRPC) ProcessTask(task shared.Task, result *shared.Result) error {
	*result = w.worker.ProcessTask(task)
	return nil
}

func main() {
	// Get coordinator's IP from command line or use default
	coordinatorAddr := shared.LocalCoordinatorAddr
	if len(os.Args) > 1 {
		// Append port if not provided
		addr := os.Args[1]
		if _, _, err := net.SplitHostPort(addr); err != nil {
			// If no port is specified, append the default port
			addr = addr + ":" + shared.CoordinatorPort
		}
		coordinatorAddr = addr
	}

	worker := NewWorker("worker1", coordinatorAddr)
	log.Printf("Worker %s: Starting up...", worker.ID)
	log.Printf("Worker %s: Connecting to coordinator at %s", worker.ID, coordinatorAddr)

	// Start RPC server
	workerRPC := &WorkerRPC{worker: worker}
	server := rpc.NewServer()
	err := server.RegisterName("Worker", workerRPC)
	if err != nil {
		log.Fatal("Failed to register RPC server:", err)
	}

	// Listen on all interfaces with a random port
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatal("Failed to start listener:", err)
	}

	// Get the actual address we're listening on
	workerAddr := listener.Addr().String()
	log.Printf("Worker listening on %s", workerAddr)

	// Try to connect to coordinator with retries
	var client *rpc.Client
	for attempts := 0; attempts < 5; attempts++ {
		log.Printf("Worker %s: Attempting to connect to coordinator (attempt %d/5)", worker.ID, attempts+1)
		client, err = rpc.Dial("tcp", worker.coordinatorAddr)
		if err == nil {
			break
		}
		log.Printf("Worker %s: Connection attempt failed: %v", worker.ID, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatal("Failed to connect to coordinator after retries:", err)
	}

	// Register with coordinator
	registration := shared.WorkerRegistration{
		ID:      worker.ID,
		Address: workerAddr,
	}

	var reply bool
	log.Printf("Worker %s: Registering with coordinator...", worker.ID)
	err = client.Call("Coordinator.RegisterWorker", registration, &reply)
	if err != nil {
		log.Fatal("Failed to register worker:", err)
	}
	client.Close()

	log.Printf("Worker %s: Successfully registered with coordinator at %s", worker.ID, worker.coordinatorAddr)

	// Start heartbeat
	go worker.startHeartbeat()

	// Serve RPC requests
	log.Printf("Worker %s: Ready to process tasks", worker.ID)
	go server.Accept(listener)

	select {} // Keep the worker running
}
