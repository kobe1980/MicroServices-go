package systemmanager

import (
	"testing"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/worker"
	"github.com/stretchr/testify/assert"
)

func TestNewSystemManager(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, err := NewSystemManager(cfg, true) // Disable metrics
	
	// Verify system manager creation
	assert.NoError(t, err)
	assert.NotNil(t, sm)
	assert.NotEmpty(t, sm.ID)
	assert.Contains(t, sm.ID, "SM") // ID should start with SM
	assert.Equal(t, cfg, sm.Config)
	assert.NotNil(t, sm.WorkersList)
	assert.NotNil(t, sm.Compressor)
	assert.NotNil(t, sm.RabbitContext)
	assert.NotNil(t, sm.Metrics)
	
	// Clean up
	sm.Kill()
}

func TestAddWorker(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Create a worker config
	workerCfg := worker.WorkerConfig{
		ID:    "test:123",
		Type:  "test",
		Tasks: []string{"task1", "task2"},
	}
	
	// Verify initial state
	assert.Empty(t, sm.WorkersList)
	
	// Add worker
	sm.AddWorker(workerCfg)
	
	// Verify worker was added
	assert.Len(t, sm.WorkersList, 1)
	assert.Equal(t, workerCfg, sm.WorkersList["test:123"])
	
	// Add another worker
	worker2Cfg := worker.WorkerConfig{
		ID:    "test:456",
		Type:  "test",
		Tasks: []string{"task3"},
	}
	
	sm.AddWorker(worker2Cfg)
	
	// Verify second worker was added
	assert.Len(t, sm.WorkersList, 2)
	assert.Equal(t, worker2Cfg, sm.WorkersList["test:456"])
}

func TestDelWorker(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Add workers
	worker1 := worker.WorkerConfig{ID: "test:123", Type: "test"}
	worker2 := worker.WorkerConfig{ID: "test:456", Type: "test"}
	
	sm.WorkersList = map[string]worker.WorkerConfig{
		"test:123": worker1,
		"test:456": worker2,
	}
	
	// Verify initial state
	assert.Len(t, sm.WorkersList, 2)
	
	// Delete first worker
	sm.DelWorker(worker1)
	
	// Verify worker was removed
	assert.Len(t, sm.WorkersList, 1)
	assert.NotContains(t, sm.WorkersList, "test:123")
	assert.Contains(t, sm.WorkersList, "test:456")
	
	// Delete non-existent worker (shouldn't error)
	sm.DelWorker(worker.WorkerConfig{ID: "nonexistent"})
	
	// Verify no change
	assert.Len(t, sm.WorkersList, 1)
}

func TestListenForJobRequest(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Add workers of different types
	sm.WorkersList = map[string]worker.WorkerConfig{
		"db:123":    {ID: "db:123", Type: "db"},
		"rest:456":  {ID: "rest:456", Type: "rest"},
		"cache:789": {ID: "cache:789", Type: "cache"},
	}
	
	// Test cases
	testCases := []struct {
		name     string
		jobData  worker.JobData
		expected bool
	}{
		{
			name: "Job for existing worker type",
			jobData: worker.JobData{
				WorkersList:   []string{"db", "rest"},
				WorkersListID: 0,
			},
			expected: true,
		},
		{
			name: "Job for next worker in list",
			jobData: worker.JobData{
				WorkersList:   []string{"db", "rest"},
				WorkersListID: 1,
			},
			expected: true,
		},
		{
			name: "Job for non-existent worker type",
			jobData: worker.JobData{
				WorkersList:   []string{"unknown"},
				WorkersListID: 0,
			},
			expected: false,
		},
		{
			name: "Job with wildcard worker type",
			jobData: worker.JobData{
				WorkersList:   []string{"db:*"},
				WorkersListID: 0,
			},
			expected: true,
		},
		{
			name: "Job with empty worker list",
			jobData: worker.JobData{
				WorkersList:   []string{},
				WorkersListID: 0,
			},
			expected: false,
		},
		{
			name: "Job with out of bounds worker list index",
			jobData: worker.JobData{
				WorkersList:   []string{"db"},
				WorkersListID: 1, // Out of bounds
			},
			expected: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sm.ListenForJobRequest(tc.jobData)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUpdateWorkerMetrics(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Add workers of different types
	sm.WorkersList = map[string]worker.WorkerConfig{
		"db:123":    {ID: "db:123", Type: "db"},
		"db:124":    {ID: "db:124", Type: "db"},
		"rest:456":  {ID: "rest:456", Type: "rest"},
		"cache:789": {ID: "cache:789", Type: "cache"},
	}
	
	// Update metrics (doesn't expose a return value, but should not error)
	sm.UpdateWorkerMetrics()
}

func TestPrintWorkersList(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Add workers
	sm.WorkersList = map[string]worker.WorkerConfig{
		"db:123":   {ID: "db:123", Type: "db"},
		"rest:456": {ID: "rest:456", Type: "rest"},
	}
	
	// Print workers list (shouldn't error)
	sm.PrintWorkersList()
}

func TestKeepAlive(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Try keep alive (should not error)
	sm.KeepAlive()
	
	// Test with nil pub (should not error)
	sm.Pub = nil
	sm.KeepAlive()
}

func TestHandleJob(t *testing.T) {
	// Create a system manager with default config
	cfg := config.DefaultConfig()
	cfg.SystemManagerLog = false // Disable logging for tests
	
	sm, _ := NewSystemManager(cfg, true) // Disable metrics
	defer sm.Kill()
	
	// Add a worker
	sm.WorkersList = map[string]worker.WorkerConfig{
		"db:123": {ID: "db:123", Type: "db"},
	}
	
	// Create a job for a supported worker type
	jobData := worker.JobData{
		WorkersList:   []string{"db"},
		WorkersListID: 0,
		Sender: worker.WorkerConfig{
			ID:   "rest:456",
			Type: "rest",
		},
		ID:   "job123",
		Data: "test data",
	}
	
	// Handle job (should not error)
	sm.handleJob(jobData)
	
	// Create a job for an unsupported worker type
	jobData2 := worker.JobData{
		WorkersList:   []string{"unknown"},
		WorkersListID: 0,
		Sender: worker.WorkerConfig{
			ID:   "rest:456",
			Type: "rest",
		},
		ID:   "job456",
		Data: "test data",
	}
	
	// Handle job (should not error)
	sm.handleJob(jobData2)
}