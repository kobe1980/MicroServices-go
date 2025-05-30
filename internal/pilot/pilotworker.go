package pilot

import (
	"fmt"
	"sync"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// PilotWorker orchestrates a pipeline of workers.
type PilotWorker struct {
	*worker.Worker
	// PendingJobs tracks running pipeline jobs by ID.
	PendingJobs map[string]worker.JobData
	mu          sync.Mutex
}

// NewPilotWorker creates a new pilot worker.
func NewPilotWorker(cfg *config.Config, metricsDisabled bool) (*PilotWorker, error) {
	base, err := worker.NewWorker("pilot", cfg, metricsDisabled)
	if err != nil {
		return nil, err
	}
	pw := &PilotWorker{
		Worker:      base,
		PendingJobs: make(map[string]worker.JobData),
	}
	return pw, nil
}

// GetConfig overrides the base worker configuration with pilot tasks.
func (w *PilotWorker) GetConfig() worker.WorkerConfig {
	cfg := w.Worker.GetConfig()
	cfg.Tasks = []string{"pipeline"}
	return cfg
}

// DoJob handles pipeline requests and responses.
func (w *PilotWorker) DoJob(job worker.JobData) {
	defer func() {
		w.SetNextJobForMe(false)
		if job.MetricTimer != nil {
			job.MetricTimer()
		}
	}()

	// Try to interpret the job data as a pipeline request.
	if reqMap, ok := job.Data.(map[string]interface{}); ok {
		if _, hasPipe := reqMap["pipeline"]; hasPipe {
			w.handlePipelineRequest(job, reqMap)
			return
		}
	}

	// Otherwise treat it as a pipeline result to forward.
	w.handlePipelineResult(job)
}

// handlePipelineRequest parses the pipeline and dispatches it to workers.
func (w *PilotWorker) handlePipelineRequest(job worker.JobData, req map[string]interface{}) {
	pipeVal, ok := req["pipeline"].([]interface{})
	if !ok {
		w.sendError(job, fmt.Errorf("invalid pipeline format"))
		return
	}

	var nextWorkers []string
	for _, step := range pipeVal {
		if s, ok := step.(string); ok {
			nextWorkers = append(nextWorkers, s)
		}
	}
	if len(nextWorkers) == 0 {
		w.sendError(job, fmt.Errorf("empty pipeline"))
		return
	}

	payload := req["data"]

	// Track the job so we can forward the response later.
	w.mu.Lock()
	w.PendingJobs[job.ID] = job
	w.mu.Unlock()

	w.SendToNextWorker(nextWorkers, payload, 0, job.ID, 1)
}

// handlePipelineResult forwards results back to the original sender if tracked.
func (w *PilotWorker) handlePipelineResult(job worker.JobData) {
	w.mu.Lock()
	orig, ok := w.PendingJobs[job.ID]
	if ok {
		delete(w.PendingJobs, job.ID)
	}
	w.mu.Unlock()

	if !ok {
		// Unknown job, just log and return.
		logger.Log("PilotWorker", w.ID, fmt.Sprintf("unexpected result for job %s", job.ID), logger.WARN)
		return
	}

	w.SendToNextWorker([]string{orig.Sender.Type}, job.Data, 0, orig.ID, 1)
}

// sendError publishes an error back to the original sender.
func (w *PilotWorker) sendError(job worker.JobData, err error) {
	logger.Log("PilotWorker", w.ID, err.Error(), logger.ERROR)
	errData := worker.Error{
		Target: job.Sender,
		Error:  err.Error(),
		ID:     job.ID,
		Data:   job.Data,
	}
	b, e := w.Compressor.Serialize(errData)
	if e != nil {
		logger.Log("PilotWorker", w.ID, fmt.Sprintf("error serializing error: %s", e.Error()), logger.ERROR)
		return
	}
	w.Pub.Publish("error", b)
}

// HandleError logs errors targeted at the pilot worker.
func (w *PilotWorker) HandleError(data worker.Error) {
	if data.Message != "" {
		logger.Log("PilotWorker", w.ID, fmt.Sprintf("error received: %s - %s", data.Error, data.Message), logger.ERROR)
	} else {
		logger.Log("PilotWorker", w.ID, fmt.Sprintf("error received: %s", data.Error), logger.ERROR)
	}
}
