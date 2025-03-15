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
	WorkersList   []string     `json:"workers_list"`
	WorkersListID int          `json:"workers_list_id"`
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
	Target WorkerConfig `json:"target"`
	Error  string       `json:"error"`
	ID     string       `json:"id"`
	Data   interface{}  `json:"data"`
}

// JobError creates a standard job error
func JobError(message string, data interface{}) Error {
	return Error{
		Error: message,
		Data:  data,
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