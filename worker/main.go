package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"matrix-operations/shared"
	"net"
	"net/rpc"
	"os"
	"time"
)

const maxRetries = 5

type Worker struct {
	ID              string
	coordinatorAddr string
	client          *rpc.Client
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
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		conn, err := tls.Dial("tcp", w.coordinatorAddr, config)
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

func main() {
	coordinatorAddr := shared.LocalCoordinatorAddr
	if len(os.Args) > 1 {
		coordinatorAddr = os.Args[1]
		if _, _, err := net.SplitHostPort(coordinatorAddr); err != nil {
			coordinatorAddr = coordinatorAddr + ":" + shared.CoordinatorPort
		}
	}

	// Load certificates
	cert, err := tls.LoadX509KeyPair("./certs/worker.crt", "./certs/worker.key")
	if err != nil {
		log.Fatalf("Failed to load worker certificate and key: %v", err)
	}

	caCert, err := os.ReadFile("./certs/ca.crt")
	if err != nil {
		log.Fatalf("Failed to load CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatal("Failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}

	worker := NewWorker("worker1", coordinatorAddr)

	// Update the connection logic to use the TLS config
	for attempts := 1; attempts <= maxRetries; attempts++ {
		conn, err := tls.Dial("tcp", coordinatorAddr, tlsConfig)
		if err != nil {
			log.Printf("Attempt %d: Failed to connect to coordinator: %v\n", attempts, err)
			if attempts == maxRetries {
				log.Fatal("Failed to connect after maximum retries")
			}
			time.Sleep(2 * time.Second)
			continue
		}
		worker.client = rpc.NewClient(conn)
		break
	}

	workerRPC := &WorkerRPC{worker: worker}
	server := rpc.NewServer()
	err = server.RegisterName("Worker", workerRPC)
	if err != nil {
		log.Fatal("Failed to register RPC server:", err)
	}

	// Create listener with proper TLS config
	listener, err := tls.Listen("tcp", "0.0.0.0:0", &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientCAs:          caCertPool,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Fatal("Failed to start TLS listener:", err)
	}

	workerAddr := listener.Addr().String()
	log.Printf("Worker listening on %s", workerAddr)

	registration := shared.WorkerRegistration{
		ID:      worker.ID,
		Address: workerAddr,
	}

	var reply bool
	err = worker.client.Call("Coordinator.RegisterWorker", registration, &reply)
	if err != nil {
		log.Fatal("Failed to register worker:", err)
	}

	log.Printf("Successfully registered with coordinator at %s", worker.coordinatorAddr)

	go worker.startHeartbeat(cert, caCertPool)
	go server.Accept(listener)

	select {} // Keep the worker running
}
