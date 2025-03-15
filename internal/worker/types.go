package worker

import (
	"fmt"
	"time"
)

// WorkerConfig represents the configuration of a worker
type WorkerConfig struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Tasks []string `json:"tasks"`
}

// JobData represents a job to be processed
type JobData struct {
	WorkersList   []string     `json:"workersList"`
	WorkersListID int          `json:"workersListId"`
	Data          interface{}  `json:"data"`
	Sender        WorkerConfig `json:"sender"`
	ID            string       `json:"id"`
	IgnoreUpdate  bool         `json:"ignoreUpdate,omitempty"`
	MetricTimer   func()       `json:"-"` // Function to stop metrics timer, not serialized
}

// JobToSend represents a job that is sent and being tracked
type JobToSend struct {
	TimeoutID *time.Timer `json:"-"` // Not serialized
	Job       JobData     `json:"job"`
	Tries     int         `json:"tries"`
}

// Error represents an error message
type Error struct {
	Target  WorkerConfig `json:"target"`
	Error   string       `json:"error"`    // Error code, typically uppercase with underscores
	Message string       `json:"message"`  // Human-readable error message
	ID      string       `json:"id"`
	Data    interface{}  `json:"data"`
}

// Error codes for standardized error handling
const (
	ErrorJobMaxRetriesExceeded = "JOB_MAX_RETRIES_EXCEEDED"
	ErrorJobTimeout            = "JOB_TIMEOUT"
	ErrorJobProcessing         = "JOB_PROCESSING_ERROR"
	ErrorInvalidFormat         = "INVALID_FORMAT"
	ErrorCommunication         = "COMMUNICATION_ERROR"
)

// JobError creates a standard job error
func JobError(code string, message string, data interface{}) Error {
	return Error{
		Error:   code,
		Message: message,
		Data:    data,
	}
}

// SameTypeWorker represents a worker of the same type
type SameTypeWorker struct {
	Worker WorkerConfig `json:"worker"`
	Tasks  []string     `json:"tasks"`
}

// NewWorkerID generates a unique worker ID based on type and timestamp
func NewWorkerID(workerType string) string {
	return fmt.Sprintf("%s:%d", workerType, time.Now().UnixNano()/int64(time.Millisecond))
}