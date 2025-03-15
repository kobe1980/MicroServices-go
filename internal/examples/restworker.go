package examples

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// RESTRequest represents a REST API request
type RESTRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// RESTResponse represents a REST API response
type RESTResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       interface{}       `json:"body,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// RESTWorker extends the base Worker to handle REST API requests
type RESTWorker struct {
	*worker.Worker
	client *http.Client
}

// NewRESTWorker creates a new REST API worker
func NewRESTWorker(cfg *config.Config, metricsDisabled bool) (*RESTWorker, error) {
	// Create base worker with "rest" type
	baseWorker, err := worker.NewWorker("rest", cfg, metricsDisabled)
	if err != nil {
		return nil, fmt.Errorf("failed to create base worker: %w", err)
	}

	// Create REST worker with a custom HTTP client with timeouts
	client := &http.Client{
		Timeout: time.Second * 30, // 30 second timeout
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxConnsPerHost:     100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	restWorker := &RESTWorker{
		Worker: baseWorker,
		client: client,
	}

	return restWorker, nil
}

// GetConfig overrides the base Worker GetConfig to add REST-specific tasks
func (w *RESTWorker) GetConfig() worker.WorkerConfig {
	config := w.Worker.GetConfig()
	config.Tasks = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	return config
}

// DoJob handles REST API requests
func (w *RESTWorker) DoJob(jobData worker.JobData) {
	// Always release the job lock when we're done
	defer func() {
		w.SetNextJobForMe(false)
		
		// Stop the metric timer
		if jobData.MetricTimer != nil {
			jobData.MetricTimer()
		}
	}()

	if w.Config.WorkerLog {
		logger.Log("RESTWorker", w.ID, "Processing REST API job")
	}

	// Parse the request
	var request RESTRequest
	var response RESTResponse

	// Try to deserialize the job data
	requestData, ok := jobData.Data.(map[string]interface{})
	if !ok {
		err := fmt.Errorf("invalid job data format")
		w.handleError(jobData, err)
		return
	}

	// Convert map to RESTRequest
	method, ok := requestData["method"].(string)
	if !ok {
		err := fmt.Errorf("missing or invalid method")
		w.handleError(jobData, err)
		return
	}
	request.Method = method

	url, ok := requestData["url"].(string)
	if !ok {
		err := fmt.Errorf("missing or invalid URL")
		w.handleError(jobData, err)
		return
	}
	request.URL = url

	// Optional fields
	if headers, ok := requestData["headers"].(map[string]interface{}); ok {
		request.Headers = make(map[string]string)
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				request.Headers[k] = strVal
			}
		}
	}

	if body, ok := requestData["body"]; ok {
		request.Body = body
	}

	// Perform the REST request
	resp, err := w.performRequest(request)
	if err != nil {
		w.handleError(jobData, err)
		return
	}

	// Send the response back to the client
	w.sendResponse(jobData, resp)
}

// performRequest executes the HTTP request
func (w *RESTWorker) performRequest(request RESTRequest) (RESTResponse, error) {
	var response RESTResponse
	var req *http.Request
	var err error

	// Create the HTTP request
	if request.Body != nil {
		// Convert body to JSON
		bodyBytes, err := json.Marshal(request.Body)
		if err != nil {
			return response, fmt.Errorf("error marshaling request body: %w", err)
		}
		
		// Create request with body using bytes.NewReader
		bodyReader := bytes.NewReader(bodyBytes)
		req, err = http.NewRequest(request.Method, request.URL, bodyReader)
	} else {
		// Create request without body
		req, err = http.NewRequest(request.Method, request.URL, nil)
	}

	if err != nil {
		return response, fmt.Errorf("error creating HTTP request: %w", err)
	}

	// Add headers to the request
	for k, v := range request.Headers {
		req.Header.Add(k, v)
	}

	// If no content-type is provided and we have a body, default to application/json
	if request.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Record metric before sending
	w.Metrics.RecordMessageSent("rest_request")

	// Execute the request
	resp, err := w.client.Do(req)
	if err != nil {
		return response, fmt.Errorf("error executing HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, fmt.Errorf("error reading response body: %w", err)
	}

	// Try to parse the response as JSON if it has the appropriate content type
	contentType := resp.Header.Get("Content-Type")
	var bodyObject interface{} = string(bodyBytes)
	
	if contentType == "application/json" || contentType == "application/javascript" {
		var jsonBody interface{}
		if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
			bodyObject = jsonBody
		}
	}

	// Build the response object
	response.Status = resp.StatusCode
	response.StatusText = resp.Status
	response.Headers = make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			response.Headers[k] = v[0]
		}
	}
	response.Body = bodyObject

	return response, nil
}

// handleError processes REST operation errors
func (w *RESTWorker) handleError(jobData worker.JobData, err error) {
	logger.Log("RESTWorker", w.ID, fmt.Sprintf("REST error: %s", err.Error()), logger.ERROR)
	
	// Record error metric
	w.Metrics.RecordError("rest_operation_error")

	// Create error message
	errorData := worker.Error{
		Target: jobData.Sender,
		Error:  err.Error(),
		ID:     jobData.ID,
		Data:   jobData.Data,
	}

	// Send error back to original sender
	errorBytes, err := w.Compressor.Serialize(errorData)
	if err != nil {
		logger.Log("RESTWorker", w.ID, fmt.Sprintf("Error serializing error response: %s", err.Error()), logger.ERROR)
		return
	}

	// Publish error
	err = w.Pub.Publish("error", errorBytes)
	if err != nil {
		logger.Log("RESTWorker", w.ID, fmt.Sprintf("Error publishing error response: %s", err.Error()), logger.ERROR)
		return
	}

	// Record error message metric
	w.Metrics.RecordMessageSent("error_response")
}

// sendResponse sends the result back to the original sender
func (w *RESTWorker) sendResponse(jobData worker.JobData, response RESTResponse) {
	// Create a response worker list with just the original sender
	responseWorkerList := []string{jobData.Sender.Type}

	// Send response
	w.SendToNextWorker(
		responseWorkerList,
		response,
		0,
		"",
		1,
	)

	if w.Config.WorkerLog {
		logger.Log("RESTWorker", w.ID, "REST job completed successfully")
	}
}

// HandleError handles errors specific to the REST worker
func (w *RESTWorker) HandleError(errorData worker.Error) {
	// Override base implementation
	if errorData.Message != "" {
		logger.Log("RESTWorker", w.ID, fmt.Sprintf("REST error received: %s - %s", errorData.Error, errorData.Message), logger.ERROR)
	} else {
		logger.Log("RESTWorker", w.ID, fmt.Sprintf("REST error received: %s", errorData.Error), logger.ERROR)
	}
	
	// Record error metric with specific type
	w.Metrics.RecordError("rest_error_received")
}