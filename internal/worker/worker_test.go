package worker

import (
	"testing"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, err := NewWorker("test", cfg, true) // Disable metrics
	
	// Verify worker creation
	assert.NoError(t, err)
	assert.NotNil(t, w)
	assert.Equal(t, "test", w.Type)
	assert.NotEmpty(t, w.ID)
	assert.Contains(t, w.ID, "test:") // ID should start with worker type
	assert.Equal(t, cfg, w.Config)
	assert.Equal(t, cfg.JobRetry, w.JobRetry)
	assert.False(t, w.NextJobForMe)
	assert.NotNil(t, w.Compressor)
	assert.NotNil(t, w.RabbitContext)
	assert.NotNil(t, w.Metrics)
	
	// Clean up
	w.Kill()
}

func TestGetConfig(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Get worker config
	workerConfig := w.GetConfig()
	
	// Verify config contents
	assert.Equal(t, w.ID, workerConfig.ID)
	assert.Equal(t, "test", workerConfig.Type)
	assert.Empty(t, workerConfig.Tasks)
}

func TestWorkerSetNextJobForMe(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Verify initial state
	assert.False(t, w.NextJobForMe)
	
	// Set to true
	w.SetNextJobForMe(true)
	assert.True(t, w.NextJobForMe)
	
	// Set back to false
	w.SetNextJobForMe(false)
	assert.False(t, w.NextJobForMe)
}

func TestWorkerIsJobForMe(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Test cases
	testCases := []struct {
		name     string
		jobData  JobData
		expected bool
	}{
		{
			name: "Job for this worker type",
			jobData: JobData{
				Sender: WorkerConfig{ID: "other:123", Type: "other"},
				WorkersList: []string{"test"},
				WorkersListID: 0,
			},
			expected: true,
		},
		{
			name: "Job for wildcard worker type",
			jobData: JobData{
				Sender: WorkerConfig{ID: "other:123", Type: "other"},
				WorkersList: []string{"test:*"},
				WorkersListID: 0,
			},
			expected: true,
		},
		{
			name: "Job for different worker type",
			jobData: JobData{
				Sender: WorkerConfig{ID: "other:123", Type: "other"},
				WorkersList: []string{"different"},
				WorkersListID: 0,
			},
			expected: false,
		},
		{
			name: "Job from self",
			jobData: JobData{
				Sender: WorkerConfig{ID: w.ID, Type: "test"},
				WorkersList: []string{"test"},
				WorkersListID: 0,
			},
			expected: false,
		},
		{
			name: "Invalid worker list index",
			jobData: JobData{
				Sender: WorkerConfig{ID: "other:123", Type: "other"},
				WorkersList: []string{"test"},
				WorkersListID: 1, // Out of bounds
			},
			expected: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset worker state
			w.SetNextJobForMe(false)
			
			// Check if job is for this worker
			result := w.IsJobForMe(tc.jobData)
			assert.Equal(t, tc.expected, result)
			
			// If job should be accepted, verify NextJobForMe was set
			if tc.expected {
				assert.True(t, w.NextJobForMe)
			} else {
				assert.False(t, w.NextJobForMe)
			}
		})
	}
}

func TestWorkerUpdateSameTypeWorkers(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Mock same type workers list with this worker as first
	w.SameTypeWorkers = []SameTypeWorker{
		{
			Worker: WorkerConfig{ID: w.ID, Type: "test"},
			Tasks:  []string{},
		},
		{
			Worker: WorkerConfig{ID: "test:456", Type: "test"},
			Tasks:  []string{},
		},
	}
	
	// Call update function - doesn't error but won't actually send since
	// RabbitMQ isn't connected in tests
	w.UpdateSameTypeWorkers()
}

func TestWorkerNewWorker(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Add a new worker
	newWorkerCfg := WorkerConfig{
		ID:    "test:456",
		Type:  "test",
		Tasks: []string{"task1", "task2"},
	}
	
	// Verify initial state
	assert.Empty(t, w.SameTypeWorkers)
	
	// Add worker
	w.NewWorker(newWorkerCfg)
	
	// Verify worker was added
	assert.Len(t, w.SameTypeWorkers, 1)
	assert.Equal(t, newWorkerCfg, w.SameTypeWorkers[0].Worker)
	assert.Equal(t, newWorkerCfg.Tasks, w.SameTypeWorkers[0].Tasks)
	
	// Update same worker
	updatedCfg := WorkerConfig{
		ID:    "test:456",
		Type:  "test",
		Tasks: []string{"task3"},
	}
	
	w.NewWorker(updatedCfg)
	
	// Verify worker was updated, not added
	assert.Len(t, w.SameTypeWorkers, 1)
	assert.Equal(t, updatedCfg, w.SameTypeWorkers[0].Worker)
	assert.Equal(t, updatedCfg.Tasks, w.SameTypeWorkers[0].Tasks)
}

func TestWorkerDelWorker(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Setup worker list
	w.SameTypeWorkers = []SameTypeWorker{
		{
			Worker: WorkerConfig{ID: "test:123", Type: "test"},
			Tasks:  []string{},
		},
		{
			Worker: WorkerConfig{ID: "test:456", Type: "test"},
			Tasks:  []string{},
		},
	}
	
	// Delete second worker
	w.DelWorker(WorkerConfig{ID: "test:456", Type: "test"})
	
	// Verify worker was removed
	assert.Len(t, w.SameTypeWorkers, 1)
	assert.Equal(t, "test:123", w.SameTypeWorkers[0].Worker.ID)
	
	// Delete non-existent worker (shouldn't error)
	w.DelWorker(WorkerConfig{ID: "test:999", Type: "test"})
	
	// Verify no change
	assert.Len(t, w.SameTypeWorkers, 1)
}

func TestUpdateWorkersList(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Create serialized worker list
	workersList := []SameTypeWorker{
		{
			Worker: WorkerConfig{ID: "test:123", Type: "test"},
			Tasks:  []string{"task1"},
		},
		{
			Worker: WorkerConfig{ID: "test:456", Type: "test"},
			Tasks:  []string{"task2"},
		},
	}
	
	// Serialize the list
	data, err := w.Compressor.Serialize(workersList)
	assert.NoError(t, err)
	
	// Update worker list
	w.UpdateWorkersList(data)
	
	// Verify list was updated
	assert.Len(t, w.SameTypeWorkers, 2)
	assert.Equal(t, "test:123", w.SameTypeWorkers[0].Worker.ID)
	assert.Equal(t, "test:456", w.SameTypeWorkers[1].Worker.ID)
}

func TestClearJobTimeout(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Create a job with timeout
	jobID := "job123"
	job := &JobToSend{
		Job: JobData{
			ID: jobID,
		},
		TimeoutID: time.NewTimer(1 * time.Hour).C,
	}
	
	// Add job to sent jobs
	w.JobsSent = append(w.JobsSent, job)
	
	// Clear timeout
	result := w.ClearJobTimeout(jobID, INFO)
	assert.True(t, result)
	
	// Try to clear non-existent job timeout
	result = w.ClearJobTimeout("nonexistent", INFO)
	assert.False(t, result)
}

func TestDeleteJobSent(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Create jobs
	job1 := JobData{ID: "job1"}
	job2 := JobData{ID: "job2"}
	
	// Add jobs to sent jobs
	w.JobsSent = []*JobToSend{
		{Job: job1},
		{Job: job2},
	}
	
	// Delete first job
	result := w.DeleteJobSent(job1)
	assert.True(t, result)
	assert.Len(t, w.JobsSent, 1)
	assert.Equal(t, "job2", w.JobsSent[0].Job.ID)
	
	// Delete non-existent job
	result = w.DeleteJobSent(JobData{ID: "nonexistent"})
	assert.False(t, result)
	assert.Len(t, w.JobsSent, 1)
}

func TestUpdateJobsSent(t *testing.T) {
	// Create a worker with default config
	cfg := config.DefaultConfig()
	cfg.WorkerLog = false // Disable logging for tests
	
	w, _ := NewWorker("test", cfg, true) // Disable metrics
	defer w.Kill()
	
	// Create jobs
	job1 := &JobToSend{
		Job: JobData{ID: "job1", Data: "original"},
		Tries: 1,
	}
	job2 := &JobToSend{
		Job: JobData{ID: "job2"},
		Tries: 1,
	}
	
	// Add jobs to sent jobs
	w.JobsSent = []*JobToSend{job1, job2}
	
	// Update first job
	updatedJob := &JobToSend{
		Job: JobData{ID: "job1", Data: "updated"},
		Tries: 2,
	}
	
	result := w.UpdateJobsSent(updatedJob)
	assert.True(t, result)
	assert.Equal(t, "updated", w.JobsSent[0].Job.Data)
	assert.Equal(t, 2, w.JobsSent[0].Tries)
	
	// Update non-existent job
	nonExistentJob := &JobToSend{
		Job: JobData{ID: "nonexistent"},
	}
	
	result = w.UpdateJobsSent(nonExistentJob)
	assert.False(t, result)
}

func TestNewWorkerID(t *testing.T) {
	// Generate worker IDs
	id1 := NewWorkerID("test")
	
	// Verify format
	assert.Contains(t, id1, "test:")
	
	// Generate another ID and verify uniqueness
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2 := NewWorkerID("test")
	assert.NotEqual(t, id1, id2)
}