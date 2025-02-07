package main

import (
	"log"
	"net/rpc"
	"time"

	"../shared"
)

type Worker struct {
	ID              string
	coordinatorAddr string
}

func NewWorker(id string, coordinatorAddr string) *Worker {
	return &Worker{
		ID:              id,
		coordinatorAddr: coordinatorAddr,
	}
}

// Matrix operations
func (w *Worker) Add(m1, m2 [][]float64) ([][]float64, error) {
	rows, cols := len(m1), len(m1[0])
	result := make([][]float64, rows)

	for i := range result {
		result[i] = make([]float64, cols)
		for j := range result[i] {
			result[i][j] = m1[i][j] + m2[i][j]
		}
	}
	return result, nil
}

func (w *Worker) Transpose(m [][]float64) ([][]float64, error) {
	rows, cols := len(m), len(m[0])
	result := make([][]float64, cols)

	for i := range result {
		result[i] = make([]float64, rows)
		for j := range result[i] {
			result[i][j] = m[j][i]
		}
	}
	return result, nil
}

func (w *Worker) Multiply(m1, m2 [][]float64) ([][]float64, error) {
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
	return result, nil
}

func (w *Worker) ProcessTask(task shared.Task) shared.Result {
	var result shared.Result
	result.TaskID = task.ID

	var err error
	switch task.Operation {
	case shared.Addition:
		result.Matrix, err = w.Add(task.Matrix1, task.Matrix2)
	case shared.Transpose:
		result.Matrix, err = w.Transpose(task.Matrix1)
	case shared.Multiplication:
		result.Matrix, err = w.Multiply(task.Matrix1, task.Matrix2)
	}

	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (w *Worker) startHeartbeat() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		client, err := rpc.Dial("tcp", w.coordinatorAddr)
		if err != nil {
			log.Printf("Failed to connect to coordinator: %v", err)
			continue
		}

		var reply bool
		err = client.Call("Coordinator.Heartbeat", w.ID, &reply)
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
		}
		client.Close()
	}
}

func main() {
	worker := NewWorker("worker1", "localhost:1234")

	// Register with coordinator
	client, err := rpc.Dial("tcp", worker.coordinatorAddr)
	if err != nil {
		log.Fatal("Failed to connect to coordinator:", err)
	}

	var reply bool
	err = client.Call("Coordinator.RegisterWorker", worker.ID, &reply)
	if err != nil {
		log.Fatal("Failed to register worker:", err)
	}

	// Start heartbeat
	go worker.startHeartbeat()

	// Start RPC server to receive tasks
	// TODO: Implement RPC server for receiving tasks

	select {} // Keep the worker running
}
