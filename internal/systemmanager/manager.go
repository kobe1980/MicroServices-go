package systemmanager

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kobe1980/microservices-go/internal/compressor"
	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/metrics"
	"github.com/kobe1980/microservices-go/internal/rabbit"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// SystemManager manages workers and job distribution
type SystemManager struct {
	ID                     string
	Config                 *config.Config
	WorkersList            map[string]worker.WorkerConfig
	Compressor             *compressor.Compressor
	Metrics                *metrics.Metrics
	RabbitContext          *rabbit.Context
	Pub                    *rabbit.Socket
	NotificationNewWorker  *rabbit.Socket
	NotificationDelWorker  *rabbit.Socket
	NotificationNextJob    *rabbit.Socket
	NotificationNextJobAck *rabbit.Socket
	KeepAliveTimer         *time.Ticker
	mutex                  sync.Mutex
}

// NewSystemManager creates a new system manager
func NewSystemManager(cfg *config.Config, metricsDisabled bool) (*SystemManager, error) {
	// Use default config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig("")
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Generate a unique ID
	id := fmt.Sprintf("SM%d", time.Now().UnixNano()/int64(time.Millisecond))

	sm := &SystemManager{
		ID:          id,
		Config:      cfg,
		WorkersList: make(map[string]worker.WorkerConfig),
		Compressor:  compressor.NewCompressor(cfg),
	}

	// Initialize metrics
	sm.Metrics = metrics.InitMetrics("system-manager", cfg.MetricsPort, metricsDisabled)

	// Log startup
	if cfg.SystemManagerLog {
		logger.Log("SystemManager", sm.ID, "Starting server")
		logger.Log("SystemManager", sm.ID, "Starting SystemManager")
	}

	// Connect to RabbitMQ
	sm.RabbitContext = rabbit.NewContext(cfg.BrokerURL)

	// Set up manager when connection is ready
	sm.RabbitContext.OnReady(func() {
		sm.setupManager()
	})

	return sm, nil
}

// setupManager sets up the manager once the connection is ready
func (sm *SystemManager) setupManager() {
	var err error

	// Set up publish socket
	sm.Pub, err = sm.RabbitContext.NewSocket(rabbit.PUB)
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error creating pub socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect publish socket
	err = sm.Pub.Connect("notifications", "", func() {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "Connected to notifications, ready to send messages")
		}
		
		// Set up periodic keep alive checks
		sm.KeepAliveTimer = time.NewTicker(time.Duration(sm.Config.Keepalive) * time.Millisecond)
		go func() {
			for range sm.KeepAliveTimer.C {
				sm.KeepAlive()
			}
		}()
	})
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error connecting pub socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Set up new worker subscription
	sm.NotificationNewWorker, err = sm.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error creating new worker sub socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect new worker subscription
	err = sm.NotificationNewWorker.Connect("notifications", "worker.new.*", func() {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "Connected to notifications, Topic worker.new.*")
		}

		// Handle new worker messages
		err := sm.NotificationNewWorker.On("data", func(data []byte) {
			var workerData worker.WorkerConfig
			if err := sm.Compressor.Deserialize(data, &workerData); err != nil {
				logger.Log("SystemManager", sm.ID, 
					fmt.Sprintf("Error deserializing worker data: %s", err.Error()), logger.ERROR)
				return
			}

			// Process new worker
			sm.AddWorker(workerData)
		})
		if err != nil {
			logger.Log("SystemManager", sm.ID, 
				fmt.Sprintf("Error setting up new worker handler: %s", err.Error()), logger.ERROR)
			return
		}
	})
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error connecting new worker sub: %s", err.Error()), logger.ERROR)
		return
	}

	// Set up next job subscription
	sm.NotificationNextJob, err = sm.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error creating next job sub socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect next job subscription
	err = sm.NotificationNextJob.Connect("notifications", "worker.next", func() {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "Connected to notifications, Topic worker.next")
		}

		// Handle next job messages
		err := sm.NotificationNextJob.On("data", func(data []byte) {
			var jobData worker.JobData
			if err := sm.Compressor.Deserialize(data, &jobData); err != nil {
				logger.Log("SystemManager", sm.ID, 
					fmt.Sprintf("Error deserializing job data: %s", err.Error()), logger.ERROR)
				return
			}

			// Process new job
			sm.handleJob(jobData)
		})
		if err != nil {
			logger.Log("SystemManager", sm.ID, 
				fmt.Sprintf("Error setting up next job handler: %s", err.Error()), logger.ERROR)
			return
		}
	})
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error connecting next job sub: %s", err.Error()), logger.ERROR)
		return
	}

	// Set up next job ack subscription
	sm.NotificationNextJobAck, err = sm.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error creating next job ack sub socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect next job ack subscription
	err = sm.NotificationNextJobAck.Connect("notifications", "worker.next.ack", func() {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "Connected to notifications, Topic worker.next.ack")
		}
	})
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error connecting next job ack sub: %s", err.Error()), logger.ERROR)
		return
	}
}

// KeepAlive sends a keepalive message to all workers
func (sm *SystemManager) KeepAlive() {
	if sm.Pub == nil {
		return
	}

	// Record keepalive metric
	sm.Metrics.RecordMessageSent("keepalive")

	// Publish keepalive message
	err := sm.Pub.Publish("worker.getAll", "{}")
	if err != nil {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Error publishing keepalive: %s", err.Error()), logger.ERROR)
	}
}

// handleJob processes a new job
func (sm *SystemManager) handleJob(jobData worker.JobData) {
	if sm.Config.SystemManagerLog {
		jsonData, _ := json.Marshal(jobData)
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("A new task is waiting for a worker: %s", string(jsonData)))
	}

	// Record metrics
	sm.Metrics.RecordMessageReceived("job_request")
	timer := sm.Metrics.StartJobTimer("job_validation")
	defer timer()

	// Check if a worker can handle this job
	if sm.ListenForJobRequest(jobData) {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "Next Job can be done by at least one worker")
		}
	} else {
		if sm.Config.SystemManagerLog {
			logger.Log("SystemManager", sm.ID, "No worker of the good type")
		}

		// Create error response
		errorData := worker.Error{
			Target: jobData.Sender,
			Error:  "No worker available for this job",
			ID:     jobData.ID,
			Data:   jobData.Data,
		}

		// Serialize error
		errorBytes, err := sm.Compressor.Serialize(errorData)
		if err != nil {
			logger.Log("SystemManager", sm.ID, 
				fmt.Sprintf("Error serializing error data: %s", err.Error()), logger.ERROR)
			return
		}

		// Publish error
		err = sm.Pub.Publish("error", errorBytes)
		if err != nil {
			logger.Log("SystemManager", sm.ID, 
				fmt.Sprintf("Error publishing error: %s", err.Error()), logger.ERROR)
		}

		// Record error metric
		sm.Metrics.RecordError("no_worker_available")
	}
}

// ListenForJobRequest checks if a worker can handle the given job
func (sm *SystemManager) ListenForJobRequest(jobData worker.JobData) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// If job list is empty or index is out of bounds, return false
	if len(jobData.WorkersList) == 0 || jobData.WorkersListID >= len(jobData.WorkersList) {
		return false
	}

	// Get worker type from list
	workerType := jobData.WorkersList[jobData.WorkersListID]

	// Check if we have a worker of that type
	for _, w := range sm.WorkersList {
		if w.Type == workerType || workerType == w.Type+":*" {
			return true
		}
	}

	return false
}

// AddWorker adds a worker to the system
func (sm *SystemManager) AddWorker(workerData worker.WorkerConfig) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Add worker to the list
	sm.WorkersList[workerData.ID] = workerData

	if sm.Config.SystemManagerLog {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("New worker added: %s of type %s", workerData.ID, workerData.Type))
	}

	// Record metrics
	sm.Metrics.RecordMessageReceived("worker_registration")
	sm.UpdateWorkerMetrics()
}

// DelWorker removes a worker from the system
func (sm *SystemManager) DelWorker(workerData worker.WorkerConfig) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Remove worker from the list
	delete(sm.WorkersList, workerData.ID)

	if sm.Config.SystemManagerLog {
		logger.Log("SystemManager", sm.ID, 
			fmt.Sprintf("Worker deleted: %s", workerData.ID))
	}

	// Record metrics
	sm.Metrics.RecordMessageReceived("worker_deletion")
	sm.UpdateWorkerMetrics()
}

// UpdateWorkerMetrics updates the metrics for workers
func (sm *SystemManager) UpdateWorkerMetrics() {
	// Count workers by type
	workerCounts := make(map[string]int)

	for _, w := range sm.WorkersList {
		workerCounts[w.Type]++
	}

	// Update metrics
	for workerType, count := range workerCounts {
		sm.Metrics.SetWorkerCount(workerType, count)
	}

	// Update total connected workers
	sm.Metrics.SetConnectedWorkers(len(sm.WorkersList))
}

// PrintWorkersList prints the current list of workers
func (sm *SystemManager) PrintWorkersList() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Record metrics
	sm.Metrics.RecordMessageReceived("workers_list_request")
	sm.UpdateWorkerMetrics()

	if sm.Config.SystemManagerLog {
		logger.Log("SystemManager", sm.ID, fmt.Sprintf("Workers list (%d):", len(sm.WorkersList)))
		
		for id, w := range sm.WorkersList {
			logger.Log("SystemManager", sm.ID, fmt.Sprintf("Worker %s: %s", id, w.Type))
		}
	}
}

// Kill stops the system manager
func (sm *SystemManager) Kill() {
	if sm.Config.SystemManagerLog {
		logger.Log("SystemManager", sm.ID, "Stopping SystemManager")
	}

	// Stop keepalive timer
	if sm.KeepAliveTimer != nil {
		sm.KeepAliveTimer.Stop()
	}

	// Close all sockets
	if sm.Pub != nil {
		sm.Pub.Close()
	}
	if sm.NotificationNewWorker != nil {
		sm.NotificationNewWorker.Close()
	}
	if sm.NotificationDelWorker != nil {
		sm.NotificationDelWorker.Close()
	}
	if sm.NotificationNextJob != nil {
		sm.NotificationNextJob.Close()
	}
	if sm.NotificationNextJobAck != nil {
		sm.NotificationNextJobAck.Close()
	}

	// Close connection
	if sm.RabbitContext != nil {
		sm.RabbitContext.Close()
	}

	// Close metrics
	if sm.Metrics != nil {
		sm.Metrics.Close()
	}
}