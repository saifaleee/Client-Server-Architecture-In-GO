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

func (w *Worker) startHeartbeat(cert tls.Certificate, caCertPool *x509.CertPool) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		conn, err := tls.Dial("tcp", w.coordinatorAddr, &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
		})
		if err != nil {
			log.Printf("Failed to connect to coordinator: %v", err)
			continue
		}
		client := rpc.NewClient(conn)

		var reply bool
		err = client.Call("Coordinator.Heartbeat", w.ID, &reply)
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
		}
		client.Close()
	}
}

func (w *WorkerRPC) ProcessTask(task shared.Task, result *shared.Result) error {
	*result = w.worker.ProcessTask(task)
	return nil
}

func loadTLSConfig() (tls.Certificate, *x509.CertPool, error) {
	// Load the worker's certificate and private key
	cert, err := tls.LoadX509KeyPair("server.pem", "server.pem")
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to load worker certificate: %v", err)
	}

	// Load the CA certificate
	caCert, err := ioutil.ReadFile("server.pem")
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to load CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return cert, caCertPool, nil
}

func main() {
	coordinatorAddr := shared.LocalCoordinatorAddr
	if len(os.Args) > 1 {
		addr := os.Args[1]
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = addr + ":" + shared.CoordinatorPort
		}
		coordinatorAddr = addr
	}

	worker := NewWorker("worker1", coordinatorAddr)

	// Load TLS configuration
	cert, caCertPool, err := loadTLSConfig()
	if err != nil {
		log.Fatal("TLS configuration error:", err)
	}

	workerRPC := &WorkerRPC{worker: worker}
	server := rpc.NewServer()
	err = server.RegisterName("Worker", workerRPC)
	if err != nil {
		log.Fatal("Failed to register RPC server:", err)
	}

	listener, err := tls.Listen("tcp", "0.0.0.0:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})
	if err != nil {
		log.Fatal("Failed to start TLS listener:", err)
	}

	workerAddr := listener.Addr().String()
	log.Printf("Worker listening on %s", workerAddr)

	// Try to connect to coordinator with retries
	var client *rpc.Client
	for attempts := 0; attempts < 5; attempts++ {
		conn, err := tls.Dial("tcp", worker.coordinatorAddr, &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
		})
		if err == nil {
			client = rpc.NewClient(conn)
			break
		}
		log.Printf("Attempt %d: Failed to connect to coordinator: %v", attempts+1, err)
		time.Sleep(2 * time.Second)
	}

	if client == nil {
		log.Fatal("Failed to connect to coordinator after retries")
	}

	registration := shared.WorkerRegistration{
		ID:      worker.ID,
		Address: workerAddr,
	}

	var reply bool
	err = client.Call("Coordinator.RegisterWorker", registration, &reply)
	if err != nil {
		log.Fatal("Failed to register worker:", err)
	}
	client.Close()

	log.Printf("Successfully registered with coordinator at %s", worker.coordinatorAddr)

	go worker.startHeartbeat(cert, caCertPool)

	go server.Accept(listener)

	select {} // Keep the worker running
}
