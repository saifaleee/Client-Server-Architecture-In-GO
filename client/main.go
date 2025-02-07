package main

import (
	"log"
	"net/rpc"
	"time"

	"../shared"
)

type Client struct {
	coordinatorAddr string
}

func NewClient(coordinatorAddr string) *Client {
	return &Client{
		coordinatorAddr: coordinatorAddr,
	}
}

func (c *Client) RequestComputation(operation shared.MatrixOperation, matrix1, matrix2 [][]float64) (*shared.Result, error) {
	client, err := rpc.Dial("tcp", c.coordinatorAddr)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	task := shared.Task{
		ID:        "task-" + time.Now().String(),
		Operation: operation,
		Matrix1:   matrix1,
		Matrix2:   matrix2,
	}

	var result shared.Result
	err = client.Call("Coordinator.SubmitTask", &task, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func main() {
	client := NewClient("localhost:1234")

	// Example matrices
	matrix1 := [][]float64{
		{1, 2},
		{3, 4},
	}
	matrix2 := [][]float64{
		{5, 6},
		{7, 8},
	}

	// Request addition
	result, err := client.RequestComputation(shared.Addition, matrix1, matrix2)
	if err != nil {
		log.Fatal("Computation failed:", err)
	}

	log.Printf("Result: %v", result.Matrix)
}
