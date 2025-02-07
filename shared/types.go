package shared

// MatrixOperation represents the type of operation to perform
type MatrixOperation int

const (
	Addition MatrixOperation = iota
	Transpose
	Multiplication
)

// Task represents a computation request
type Task struct {
	ID        string
	Operation MatrixOperation
	Matrix1   [][]float64
	Matrix2   [][]float64 // Only used for Addition and Multiplication
}

// Result represents the computation result
type Result struct {
	TaskID string
	Matrix [][]float64
	Error  string
}

// WorkerStatus represents the current state of a worker
type WorkerStatus struct {
	ID            string
	TaskCount     int
	LastHeartbeat int64
	Available     bool
}
