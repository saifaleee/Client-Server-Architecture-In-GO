package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"matrix-operations/shared"
)

type Client struct {
	coordinatorAddr string
}

func NewClient(coordinatorAddr string) *Client {
	return &Client{
		coordinatorAddr: coordinatorAddr,
	}
}

func (c *Client) loadTLSConfig() (*tls.Config, error) {
	// Load CA certificate
	caCert, err := ioutil.ReadFile("server.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: false,
	}, nil
}

func (c *Client) RequestComputation(operation shared.MatrixOperation, matrix1, matrix2 [][]float64) (*shared.Result, error) {
	tlsConfig, err := c.loadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %v", err)
	}

	conn, err := tls.Dial("tcp", c.coordinatorAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to coordinator: %v", err)
	}
	defer conn.Close()

	client := rpc.NewClient(conn)
	defer client.Close()

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	task := shared.Task{
		ID:        taskID,
		Operation: operation,
		Matrix1:   matrix1,
		Matrix2:   matrix2,
	}

	// Submit the task
	var submitReply bool
	err = client.Call("Coordinator.SubmitTask", task, &submitReply)
	if err != nil {
		return nil, fmt.Errorf("failed to submit task: %v", err)
	}

	// Poll for results
	for i := 0; i < 30; i++ { // Retry for 30 seconds
		var result shared.Result
		err = client.Call("Coordinator.GetResult", taskID, &result)
		if err == nil {
			return &result, nil
		}
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for result")
}

func main() {
	var serverAddr string
	if len(os.Args) > 1 {
		addr := os.Args[1]
		if _, _, err := net.SplitHostPort(addr); err != nil {
			serverAddr = addr + ":" + shared.CoordinatorPort
		} else {
			serverAddr = addr
		}
	} else {
		fmt.Println("Usage: go run main.go <server-ip>")
		fmt.Println("Example: go run main.go 192.168.1.100")
		os.Exit(1)
	}

	client := NewClient(serverAddr)
	fmt.Printf("Connecting to coordinator at %s...\n", serverAddr)

	// Test connection
	tlsConfig, err := client.loadTLSConfig()
	if err != nil {
		log.Fatalf("Failed to load TLS config: %v", err)
	}

	testConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to coordinator: %v", err)
	}
	testConn.Close()
	fmt.Println("Successfully connected to coordinator!")

	// Example matrices
	matrix1 := [][]float64{
		{1, 2},
		{3, 4},
	}
	matrix2 := [][]float64{
		{5, 6},
		{7, 8},
	}

	fmt.Println("Matrix 1:")
	printMatrix(matrix1)
	fmt.Println("\nMatrix 2:")
	printMatrix(matrix2)

	fmt.Println("\nPerforming matrix addition...")
	result, err := client.RequestComputation(shared.Addition, matrix1, matrix2)
	if err != nil {
		log.Fatal("Computation failed:", err)
	}

	fmt.Println("\nResult:")
	printMatrix(result.Matrix)
}

func printMatrix(matrix [][]float64) {
	for _, row := range matrix {
		fmt.Println(row)
	}
}
