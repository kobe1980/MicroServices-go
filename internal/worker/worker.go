// Package worker implements the base worker functionality for specialized microservices
package worker

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
)

// Worker represents a microservice worker
type Worker struct {
	Config          *config.Config
	Type            string
	ID              string
	Compressor      *compressor.Compressor
	Metrics         *metrics.Metrics
	Pub             *rabbit.Socket
	NotifErrorSub   *rabbit.Socket
	NotifNewWorker  *rabbit.Socket
	NotifWorkerList *rabbit.Socket
	NotifGetAllSub  *rabbit.Socket
	NotifNextJobSub *rabbit.Socket
	NextJobPub      *rabbit.Socket
	NextJobAckSub   *rabbit.Socket
	NextJobAckPub   *rabbit.Socket
	RabbitContext   *rabbit.Context
	JobsSent        []*JobToSend
	SameTypeWorkers []SameTypeWorker
	NextJobForMe    bool
	JobRetry        int
	mutex           sync.Mutex
}

// NewWorker creates a new worker
func NewWorker(workerType string, cfg *config.Config, metricsDisabled bool) (*Worker, error) {
	// Use default config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig("")
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	w := &Worker{
		Config:       cfg,
		Type:         workerType,
		ID:           NewWorkerID(workerType),
		Compressor:   compressor.NewCompressor(cfg),
		JobsSent:     make([]*JobToSend, 0),
		NextJobForMe: false,
		JobRetry:     cfg.JobRetry,
	}

	// Initialize metrics
	w.Metrics = metrics.InitMetrics("worker-"+workerType, 0, metricsDisabled)

	// Log worker startup
	if cfg.WorkerLog {
		logger.Log("Worker", w.ID, fmt.Sprintf("Starting client - type: %s, id: %s", workerType, w.ID))
	}

	// Connect to RabbitMQ
	w.RabbitContext = rabbit.NewContext(cfg.BrokerURL)

	// Setup connection and callbacks
	w.RabbitContext.OnReady(func() {
		// Setup pub socket
		pubSocket, err := w.RabbitContext.NewSocket(rabbit.PUB)
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error creating pub socket: %s", err.Error()), logger.ERROR)
			return
		}
		w.Pub = pubSocket

		// Connect pub socket
		err = w.Pub.Connect("notifications", "", func() {
			if cfg.WorkerLog {
				logger.Log("Worker", w.ID, "Connected to notifications")
			}
			// Serialize worker config
			configBytes, err := w.Compressor.Serialize(w.GetConfig())
			if err != nil {
				logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing worker config: %s", err.Error()), logger.ERROR)
				return
			}

			// Publish worker registration
			err = w.Pub.Publish("worker.new.send", configBytes)
			if err != nil {
				logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing worker registration: %s", err.Error()), logger.ERROR)
				return
			}

			// Record metric for worker registration
			w.Metrics.RecordMessageSent("worker_registration")
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting pub socket: %s", err.Error()), logger.ERROR)
			return
		}

		// Setup worker sockets
		w.setupWorkerSockets()
	})

	return w, nil
}

// setupWorkerSockets sets up all worker related sockets
func (w *Worker) setupWorkerSockets() {
	var err error

	// Error notifications socket
	w.NotifErrorSub, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating error subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to error notifications
	err = w.NotifErrorSub.Connect("notifications", "error", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notification, Topic error")
		}

		// Handle error messages
		err := w.NotifErrorSub.On("data", func(data []byte) {
			// Record error metric
			w.Metrics.RecordMessageReceived("error")
			
			// Process error
			w.ReceiveError(data)
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up error handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting error subscription: %s", err.Error()), logger.ERROR)
	}

	// Get all workers notifications socket
	w.NotifGetAllSub, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating getAll subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to getAll notifications
	err = w.NotifGetAllSub.Connect("notifications", "worker.getAll", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notifications, Topic worker.getAll")
		}

		// Handle getAll messages
		err := w.NotifGetAllSub.On("data", func(data []byte) {
			// Serialize worker config
			configBytes, err := w.Compressor.Serialize(w.GetConfig())
			if err != nil {
				logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing worker config: %s", err.Error()), logger.ERROR)
				return
			}

			// Publish worker response
			err = w.Pub.Publish("worker.new.resend", configBytes)
			if err != nil {
				logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing worker response: %s", err.Error()), logger.ERROR)
				return
			}

			// Record keepalive response metric
			w.Metrics.RecordMessageSent("keepalive_response")
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up getAll handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting getAll subscription: %s", err.Error()), logger.ERROR)
	}

	// Next job notifications socket
	w.NotifNextJobSub, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating next job subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to next job notifications
	err = w.NotifNextJobSub.Connect("notifications", "worker.next", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notifications, Topic worker.next")
		}

		// Handle next job messages
		err := w.NotifNextJobSub.On("data", func(data []byte) {
			// Record job request metric
			w.Metrics.RecordMessageReceived("job_request")
			
			// Process job request
			w.ReceiveNextJob(data)
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up next job handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting next job subscription: %s", err.Error()), logger.ERROR)
	}

	// New worker notifications socket
	w.NotifNewWorker, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating new worker subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to new worker notifications
	err = w.NotifNewWorker.Connect("notifications", "worker.new.*", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notifications, Topic worker.new.*")
		}

		// Handle new worker messages
		err := w.NotifNewWorker.On("data", func(data []byte) {
			// Process new worker
			w.HandleWorkerEvent(data)
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up new worker handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting new worker subscription: %s", err.Error()), logger.ERROR)
	}

	// Worker list notifications socket
	w.NotifWorkerList, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating worker list subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to worker list notifications
	err = w.NotifWorkerList.Connect("notifications", "worker.list", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notifications, Topic worker.list")
		}

		// Handle worker list messages
		err := w.NotifWorkerList.On("data", func(data []byte) {
			// Process worker list
			w.UpdateWorkersList(data)
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up worker list handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting worker list subscription: %s", err.Error()), logger.ERROR)
	}

	// Next job acknowledgment socket
	w.NextJobAckSub, err = w.RabbitContext.NewSocket(rabbit.SUB)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error creating next job ack subscription socket: %s", err.Error()), logger.ERROR)
		return
	}

	// Connect to next job acknowledgment notifications
	err = w.NextJobAckSub.Connect("notifications", "worker.next.ack", func() {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Connected to notifications, Topic worker.next.ack")
		}

		// Handle next job ack messages
		err := w.NextJobAckSub.On("data", func(data []byte) {
			// Process job ack
			w.ReceiveNextJobAck(data)
		})
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error setting up next job ack handler: %s", err.Error()), logger.ERROR)
		}
	})
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error connecting next job ack subscription: %s", err.Error()), logger.ERROR)
	}
}

// GetConfig returns the worker configuration
func (w *Worker) GetConfig() WorkerConfig {
	return WorkerConfig{
		ID:    w.ID,
		Type:  w.Type,
		Tasks: []string{},
	}
}

// ReceiveError handles error messages
func (w *Worker) ReceiveError(data []byte) {
	var errorData Error
	if err := w.Compressor.Deserialize(data, &errorData); err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error deserializing error data: %s", err.Error()), logger.ERROR)
		return
	}

	// Only process errors targeted at this worker
	if errorData.Target.ID == w.ID {
		w.HandleError(errorData)
	}
}

// HandleError processes error messages - to be overridden by implementations
// This method name is aligned with JavaScript convention for error handling
func (w *Worker) HandleError(data Error) error {
	// Base implementation just logs the error
	if data.Message != "" {
		logger.Log("Worker", w.ID, fmt.Sprintf("Received error: %s - %s", data.Error, data.Message), logger.ERROR)
	} else {
		logger.Log("Worker", w.ID, fmt.Sprintf("Received error: %s", data.Error), logger.ERROR)
	}
	return nil
}

// ReceiveNextJob handles next job messages
func (w *Worker) ReceiveNextJob(data []byte) {
	var jobData JobData
	if err := w.Compressor.Deserialize(data, &jobData); err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error deserializing job data: %s", err.Error()), logger.ERROR)
		return
	}

	// Check if this job is for us
	if w.IsJobForMe(jobData) {
		w.ActivateJob(jobData, false)
	}
}

// IsJobForMe checks if the job is for this worker
func (w *Worker) IsJobForMe(jobData JobData) bool {
	// Skip if job came from us
	if jobData.Sender.ID == w.ID {
		return false
	}

	// Get current worker in the list
	if jobData.WorkersListID >= len(jobData.WorkersList) {
		return false
	}

	workerType := jobData.WorkersList[jobData.WorkersListID]

	// Check if this job matches our worker type
	if workerType == w.Type || workerType == w.Type+":*" {
		w.mutex.Lock()
		defer w.mutex.Unlock()
		
		// Only take the job if we're not busy
		if !w.NextJobForMe {
			w.NextJobForMe = true
			return true
		}
	}

	return false
}

// ActivateJob processes an incoming job
func (w *Worker) ActivateJob(jobData JobData, ignoreUpdate bool) {
	if w.Config.WorkerLog {
		jsonData, _ := json.Marshal(jobData)
		logger.Log("Worker", w.ID, fmt.Sprintf("Receiving next job: %s", string(jsonData)))
	}

	// Record metric for received job
	w.Metrics.RecordMessageReceived("job_activated")

	// Check if we need to pass to the next worker in the list
	if len(jobData.WorkersList) > (jobData.WorkersListID+1) {
		jobData.WorkersListID = (jobData.WorkersListID + 1)
	}

	// Set ignore update flag if requested
	if ignoreUpdate {
		jobData.IgnoreUpdate = ignoreUpdate
	}

	// Serialize job acknowledgment
	ackBytes, err := w.Compressor.Serialize(jobData)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing job ack: %s", err.Error()), logger.ERROR)
		return
	}

	// Publish job acknowledgment
	err = w.Pub.Publish("worker.next.ack", ackBytes)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing job ack: %s", err.Error()), logger.ERROR)
		return
	}

	// Start timing the job processing
	jobData.MetricTimer = w.Metrics.StartJobTimer("job_processing")

	// Process the job
	w.DoJob(jobData)
}

// DoJob processes a job - to be overridden by implementations
func (w *Worker) DoJob(jobData JobData) {
	// Base implementation just releases the job lock
	w.mutex.Lock()
	w.NextJobForMe = false
	w.mutex.Unlock()

	// Stop the timer if it exists
	if jobData.MetricTimer != nil {
		jobData.MetricTimer()
	}
}

// ReceiveNextJobAck handles job acknowledgment messages
func (w *Worker) ReceiveNextJobAck(data []byte) {
	var jobData JobData
	if err := w.Compressor.Deserialize(data, &jobData); err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error deserializing job ack: %s", err.Error()), logger.ERROR)
		return
	}

	// Only process acks for jobs we sent
	if jobData.Sender.ID == w.ID {
		if w.Config.WorkerLog {
			jsonData, _ := json.Marshal(jobData)
			logger.Log("Worker", w.ID, fmt.Sprintf("Receiving next job Ack: %s", string(jsonData)))
		}

		// Clear job timeout
		w.ClearJobTimeout(jobData.ID, logger.INFO)
		
		// Delete job from sent jobs
		w.DeleteJobSent(jobData)

		// Record job acknowledgment metric
		w.Metrics.RecordMessageReceived("job_ack")

		// Update same type workers if not ignored
		if !jobData.IgnoreUpdate {
			w.UpdateSameTypeWorkers()
		}
	}
}

// SendToNextWorker sends a job to the next appropriate worker
func (w *Worker) SendToNextWorker(nextWorkers []string, data interface{}, workersListID int, jobID string, tries int) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Generate job ID if not provided
	if jobID == "" {
		jobID = fmt.Sprintf("J%d", time.Now().UnixNano()/int64(time.Millisecond))
	}

	// Create the job
	jobToSend := &JobToSend{
		Job: JobData{
			WorkersList:   nextWorkers,
			WorkersListID: workersListID,
			Data:          data,
			Sender:        w.GetConfig(),
			ID:            jobID,
		},
		Tries: tries,
	}

	// Serialize job
	jobBytes, err := w.Compressor.Serialize(jobToSend.Job)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing job: %s", err.Error()), logger.ERROR)
		return
	}

	// Publish job
	if w.Config.WorkerLog {
		logger.Log("Worker", w.ID, "Sending data to the next worker on the list.")
	}
	
	err = w.Pub.Publish("worker.next", jobBytes)
	if err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing job: %s", err.Error()), logger.ERROR)
		return
	}

	// Record metrics for sent messages
	w.Metrics.RecordMessageSent("job_request")

	// Set timeout for job
	jobToSend.TimeoutID = time.AfterFunc(
		time.Duration(w.Config.JobTimeout)*time.Millisecond,
		func() { w.Resend(nextWorkers, jobToSend) },
	)

	// Add to sent jobs if not a retry
	if tries <= 1 {
		w.JobsSent = append(w.JobsSent, jobToSend)
	} else {
		w.UpdateJobsSent(jobToSend)
	}
}

// Resend resends a job if no acknowledgment was received
func (w *Worker) Resend(nextWorkers []string, jobToSend *JobToSend) {
	if w.Config.WorkerLog {
		logger.Log("Worker", w.ID, "No worker took the job, resending it")
	}

	// Record job retry metric
	w.Metrics.RecordError("job_retry")

	// Check if max retries reached
	if jobToSend.Tries >= w.JobRetry {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, 
				fmt.Sprintf("Job Send %d times. Stopping", w.JobRetry), logger.ERROR)
		}
		
		// Record error metric
		w.Metrics.RecordError("job_max_retries_exceeded")
		
		// Treat as error using standardized error format
		w.HandleError(Error{
			Error:   ErrorJobMaxRetriesExceeded,
			Message: fmt.Sprintf("Job sent %d times, maximum retries exceeded", w.JobRetry),
			Data:    jobToSend.Job.Data,
		})
	}

	// Increment tries and resend
	w.SendToNextWorker(
		nextWorkers,
		jobToSend.Job.Data,
		jobToSend.Job.WorkersListID,
		jobToSend.Job.ID,
		jobToSend.Tries+1,
	)
}

// UpdateJobsSent updates a job in the sent jobs list
func (w *Worker) UpdateJobsSent(job *JobToSend) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	for i, j := range w.JobsSent {
		if j.Job.ID == job.Job.ID {
			w.JobsSent[i] = job
			return true
		}
	}
	return false
}

// DeleteJobSent removes a job from the sent jobs list
func (w *Worker) DeleteJobSent(job JobData) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	for i, j := range w.JobsSent {
		if j.Job.ID == job.ID {
			// Remove the job
			w.JobsSent = append(w.JobsSent[:i], w.JobsSent[i+1:]...)
			return true
		}
	}
	return false
}

// ClearJobTimeout clears the timeout for a job
func (w *Worker) ClearJobTimeout(jobID string, level logger.LogLevel) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	for _, j := range w.JobsSent {
		if j.Job.ID == jobID {
			// Stop the timer
			if j.TimeoutID != nil {
				j.TimeoutID.Stop()
				j.TimeoutID = nil
			}
			
			if w.Config.WorkerLog {
				logger.Log("Worker", w.ID, fmt.Sprintf("The job %s is done by a worker", jobID), level)
			}
			return true
		}
	}
	return false
}

// HandleWorkerEvent processes worker-related events
func (w *Worker) HandleWorkerEvent(data []byte) {
	var worker WorkerConfig
	if err := w.Compressor.Deserialize(data, &worker); err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error deserializing worker data: %s", err.Error()), logger.ERROR)
		return
	}

	// Only process events for workers of the same type
	if worker.Type == w.Type {
		w.NewWorker(worker)
	}
}

// NewWorker processes a new worker event
func (w *Worker) NewWorker(worker WorkerConfig) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	// Check if this worker is already in our list
	for i, stw := range w.SameTypeWorkers {
		if stw.Worker.ID == worker.ID {
			w.SameTypeWorkers[i].Worker = worker
			return
		}
	}

	// Add the new worker
	w.SameTypeWorkers = append(w.SameTypeWorkers, SameTypeWorker{
		Worker: worker,
		Tasks:  worker.Tasks,
	})

	// If we're the first in the list, broadcast the list
	if len(w.SameTypeWorkers) > 0 && 
	   w.SameTypeWorkers[0].Worker.ID == w.ID && 
	   worker.ID != w.ID {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "First on the list, sending list to others")
		}
		
		// Serialize worker list
		listBytes, err := w.Compressor.Serialize(w.SameTypeWorkers)
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing worker list: %s", err.Error()), logger.ERROR)
			return
		}
		
		// Publish worker list
		err = w.Pub.Publish("worker.list", listBytes)
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing worker list: %s", err.Error()), logger.ERROR)
			return
		}
		
		// Record worker list update metric
		w.Metrics.RecordMessageSent("worker_list_update")
	}
}

// DelWorker processes a worker removal event
func (w *Worker) DelWorker(worker WorkerConfig) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	for i, stw := range w.SameTypeWorkers {
		if stw.Worker.ID == worker.ID {
			// Remove the worker
			w.SameTypeWorkers = append(w.SameTypeWorkers[:i], w.SameTypeWorkers[i+1:]...)
			
			// Update index to broadcast
			updateIndex := i
			if updateIndex >= len(w.SameTypeWorkers) {
				updateIndex = 0
			}
			
			w.updateSameTypeWorkersWithIndex(updateIndex)
			
			// Record worker removal metric
			w.Metrics.RecordMessageReceived("worker_removal")
			break
		}
	}
}

// UpdateWorkersList updates the list of same type workers
func (w *Worker) UpdateWorkersList(data []byte) {
	var workersList []SameTypeWorker
	if err := w.Compressor.Deserialize(data, &workersList); err != nil {
		logger.Log("Worker", w.ID, fmt.Sprintf("Error deserializing workers list: %s", err.Error()), logger.ERROR)
		return
	}

	// Only update if list is for our type
	if len(workersList) > 0 && workersList[0].Worker.Type == w.Type {
		if w.Config.WorkerLog {
			logger.Log("Worker", w.ID, "Updating Workers List")
		}
		
		w.mutex.Lock()
		w.SameTypeWorkers = workersList
		w.mutex.Unlock()
		
		// Record worker list update metric
		w.Metrics.RecordMessageReceived("worker_list_update")
	}
}

// UpdateSameTypeWorkers updates the list of same type workers
func (w *Worker) UpdateSameTypeWorkers() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	w.updateSameTypeWorkersWithIndex(0)
}

// updateSameTypeWorkersWithIndex updates the list of same type workers with an index
func (w *Worker) updateSameTypeWorkersWithIndex(index int) {
	// Only update if we have workers and we're first in the list
	if len(w.SameTypeWorkers) > 0 && 
	   index < len(w.SameTypeWorkers) && 
	   w.SameTypeWorkers[index].Worker.ID == w.ID {
		
		// Serialize worker list
		listBytes, err := w.Compressor.Serialize(w.SameTypeWorkers)
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error serializing worker list: %s", err.Error()), logger.ERROR)
			return
		}
		
		// Publish worker list
		err = w.Pub.Publish("worker.list", listBytes)
		if err != nil {
			logger.Log("Worker", w.ID, fmt.Sprintf("Error publishing worker list: %s", err.Error()), logger.ERROR)
			return
		}
	}
}

// PrintSameTypeWorkers prints the list of same type workers
func (w *Worker) PrintSameTypeWorkers() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	if w.Config.WorkerLog {
		logger.Log("Worker", w.ID, fmt.Sprintf("Workers list (%d):", len(w.SameTypeWorkers)))
		
		for i, stw := range w.SameTypeWorkers {
			logger.Log("Worker", w.ID, fmt.Sprintf("Worker %d: %s", i, stw.Worker.ID))
		}
	}
}

// SetNextJobForMe sets whether this worker is ready to process a job
func (w *Worker) SetNextJobForMe(value bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	w.NextJobForMe = value
}

// Kill stops the worker
func (w *Worker) Kill() {
	if w.Config.WorkerLog {
		logger.Log("Worker", w.ID, "Stopping client")
	}
	
	// Close all sockets
	if w.Pub != nil {
		w.Pub.Close()
	}
	if w.NotifErrorSub != nil {
		w.NotifErrorSub.Close()
	}
	if w.NotifNewWorker != nil {
		w.NotifNewWorker.Close()
	}
	if w.NotifWorkerList != nil {
		w.NotifWorkerList.Close()
	}
	if w.NotifGetAllSub != nil {
		w.NotifGetAllSub.Close()
	}
	if w.NotifNextJobSub != nil {
		w.NotifNextJobSub.Close()
	}
	if w.NextJobPub != nil {
		w.NextJobPub.Close()
	}
	if w.NextJobAckSub != nil {
		w.NextJobAckSub.Close()
	}
	if w.NextJobAckPub != nil {
		w.NextJobAckPub.Close()
	}
	
	// Close connection
	if w.RabbitContext != nil {
		w.RabbitContext.Close()
	}
	
	// Close metrics
	if w.Metrics != nil {
		w.Metrics.Close()
	}
}