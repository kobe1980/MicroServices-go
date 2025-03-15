package examples

import (
	"errors"
	"fmt"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// DBRequest represents a database request
type DBRequest struct {
	Action string      `json:"action"`
	Table  string      `json:"table"`
	Query  interface{} `json:"query,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// DBResponse represents a database response
type DBResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// DBWorker extends the base Worker to handle database operations
type DBWorker struct {
	*worker.Worker
	// In a real implementation, this would be a database connection
	// For this example, we'll simulate database operations
	db map[string][]interface{}
}

// NewDBWorker creates a new database worker
func NewDBWorker(cfg *config.Config, metricsDisabled bool) (*DBWorker, error) {
	// Create base worker with "db" type
	baseWorker, err := worker.NewWorker("db", cfg, metricsDisabled)
	if err != nil {
		return nil, fmt.Errorf("failed to create base worker: %w", err)
	}

	// Create DB worker
	dbWorker := &DBWorker{
		Worker: baseWorker,
		db:     make(map[string][]interface{}),
	}

	// Initialize sample database tables
	dbWorker.db["users"] = []interface{}{
		map[string]interface{}{
			"id":   1,
			"name": "John Doe",
			"age":  30,
		},
		map[string]interface{}{
			"id":   2,
			"name": "Jane Smith",
			"age":  25,
		},
	}

	dbWorker.db["products"] = []interface{}{
		map[string]interface{}{
			"id":    1,
			"name":  "Laptop",
			"price": 999.99,
		},
		map[string]interface{}{
			"id":    2,
			"name":  "Smartphone",
			"price": 499.99,
		},
	}

	return dbWorker, nil
}

// GetConfig overrides the base Worker GetConfig to add DB-specific tasks
func (w *DBWorker) GetConfig() worker.WorkerConfig {
	config := w.Worker.GetConfig()
	config.Tasks = []string{"find", "insert", "update", "delete"}
	return config
}

// DoJob handles database operations
func (w *DBWorker) DoJob(jobData worker.JobData) {
	// Always release the job lock when we're done
	defer func() {
		w.SetNextJobForMe(false)
		
		// Stop the metric timer
		if jobData.MetricTimer != nil {
			jobData.MetricTimer()
		}
	}()

	if w.Config.WorkerLog {
		logger.Log("DBWorker", w.ID, "Processing database job")
	}

	// Parse the request
	var request DBRequest
	var response DBResponse
	var err error

	// Try to deserialize the job data
	requestData, ok := jobData.Data.(map[string]interface{})
	if !ok {
		err = errors.New("invalid job data format")
		w.handleError(jobData, err)
		return
	}

	// Convert map to DBRequest
	action, ok := requestData["action"].(string)
	if !ok {
		err = errors.New("missing or invalid action")
		w.handleError(jobData, err)
		return
	}
	request.Action = action

	table, ok := requestData["table"].(string)
	if !ok {
		err = errors.New("missing or invalid table")
		w.handleError(jobData, err)
		return
	}
	request.Table = table

	// Optional fields
	if query, ok := requestData["query"]; ok {
		request.Query = query
	}
	if data, ok := requestData["data"]; ok {
		request.Data = data
	}

	// Process the request based on action
	switch request.Action {
	case "find":
		response, err = w.handleFind(request)
	case "insert":
		response, err = w.handleInsert(request)
	case "update":
		response, err = w.handleUpdate(request)
	case "delete":
		response, err = w.handleDelete(request)
	default:
		err = fmt.Errorf("unsupported action: %s", request.Action)
	}

	// Handle any errors
	if err != nil {
		w.handleError(jobData, err)
		return
	}

	// Prepare response to send back
	w.sendResponse(jobData, response)
}

// handleFind processes a find request
func (w *DBWorker) handleFind(request DBRequest) (DBResponse, error) {
	// Check if table exists
	data, ok := w.db[request.Table]
	if !ok {
		return DBResponse{}, fmt.Errorf("table not found: %s", request.Table)
	}

	// If no query, return all data
	if request.Query == nil {
		return DBResponse{
			Status: "success",
			Data:   data,
		}, nil
	}

	// Simulate a simple query (in real DB this would be more complex)
	// For demonstration, we'll just do a simple match on id if provided
	query, ok := request.Query.(map[string]interface{})
	if !ok {
		return DBResponse{}, errors.New("invalid query format")
	}

	if id, ok := query["id"]; ok {
		// Find item by id
		for _, item := range data {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["id"] == id {
					return DBResponse{
						Status: "success",
						Data:   item,
					}, nil
				}
			}
		}
		return DBResponse{}, fmt.Errorf("no item found with id: %v", id)
	}

	// If no specific id, return all data (simplistic approach)
	return DBResponse{
		Status: "success",
		Data:   data,
	}, nil
}

// handleInsert processes an insert request
func (w *DBWorker) handleInsert(request DBRequest) (DBResponse, error) {
	// Validate required fields
	if request.Data == nil {
		return DBResponse{}, errors.New("data is required for insert operation")
	}

	// Create or access the table
	if _, ok := w.db[request.Table]; !ok {
		w.db[request.Table] = []interface{}{}
	}

	// Simulate insert (add to array)
	// In a real implementation, we'd validate the data structure, etc.
	w.db[request.Table] = append(w.db[request.Table], request.Data)

	// Add artificial delay to simulate DB operation
	time.Sleep(50 * time.Millisecond)

	return DBResponse{
		Status:  "success",
		Message: "Item inserted successfully",
		Data:    request.Data,
	}, nil
}

// handleUpdate processes an update request
func (w *DBWorker) handleUpdate(request DBRequest) (DBResponse, error) {
	// Validate required fields
	if request.Query == nil {
		return DBResponse{}, errors.New("query is required for update operation")
	}
	if request.Data == nil {
		return DBResponse{}, errors.New("data is required for update operation")
	}

	// Check if table exists
	tableData, ok := w.db[request.Table]
	if !ok {
		return DBResponse{}, fmt.Errorf("table not found: %s", request.Table)
	}

	// Simulate update operation
	query, ok := request.Query.(map[string]interface{})
	if !ok {
		return DBResponse{}, errors.New("invalid query format")
	}

	// Find by ID and update
	if id, ok := query["id"]; ok {
		for i, item := range tableData {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["id"] == id {
					// Update item with new data
					newData, ok := request.Data.(map[string]interface{})
					if !ok {
						return DBResponse{}, errors.New("invalid data format")
					}

					// In a real implementation, would be more sophisticated
					for k, v := range newData {
						itemMap[k] = v
					}

					// Update in the "database"
					w.db[request.Table][i] = itemMap

					// Add artificial delay to simulate DB operation
					time.Sleep(100 * time.Millisecond)

					return DBResponse{
						Status:  "success",
						Message: "Item updated successfully",
						Data:    itemMap,
					}, nil
				}
			}
		}
		return DBResponse{}, fmt.Errorf("no item found with id: %v", id)
	}

	return DBResponse{}, errors.New("update requires an id in the query")
}

// handleDelete processes a delete request
func (w *DBWorker) handleDelete(request DBRequest) (DBResponse, error) {
	// Validate required fields
	if request.Query == nil {
		return DBResponse{}, errors.New("query is required for delete operation")
	}

	// Check if table exists
	tableData, ok := w.db[request.Table]
	if !ok {
		return DBResponse{}, fmt.Errorf("table not found: %s", request.Table)
	}

	// Simulate delete operation
	query, ok := request.Query.(map[string]interface{})
	if !ok {
		return DBResponse{}, errors.New("invalid query format")
	}

	// Find by ID and delete
	if id, ok := query["id"]; ok {
		for i, item := range tableData {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["id"] == id {
					// Remove item from table
					w.db[request.Table] = append(w.db[request.Table][:i], w.db[request.Table][i+1:]...)

					// Add artificial delay to simulate DB operation
					time.Sleep(75 * time.Millisecond)

					return DBResponse{
						Status:  "success",
						Message: "Item deleted successfully",
					}, nil
				}
			}
		}
		return DBResponse{}, fmt.Errorf("no item found with id: %v", id)
	}

	return DBResponse{}, errors.New("delete requires an id in the query")
}

// handleError processes database operation errors
func (w *DBWorker) handleError(jobData worker.JobData, err error) {
	logger.Log("DBWorker", w.ID, fmt.Sprintf("Database error: %s", err.Error()), logger.ERROR)
	
	// Record error metric
	w.Metrics.RecordError("db_operation_error")

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
		logger.Log("DBWorker", w.ID, fmt.Sprintf("Error serializing error response: %s", err.Error()), logger.ERROR)
		return
	}

	// Publish error
	err = w.Pub.Publish("error", errorBytes)
	if err != nil {
		logger.Log("DBWorker", w.ID, fmt.Sprintf("Error publishing error response: %s", err.Error()), logger.ERROR)
		return
	}

	// Record error message metric
	w.Metrics.RecordMessageSent("error_response")
}

// sendResponse sends the result back to the original sender
func (w *DBWorker) sendResponse(jobData worker.JobData, response DBResponse) {
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
		logger.Log("DBWorker", w.ID, "Database job completed successfully")
	}
}

// HandleError handles errors specific to the DB worker
func (w *DBWorker) HandleError(errorData worker.Error) {
	// Override base implementation
	if errorData.Message != "" {
		logger.Log("DBWorker", w.ID, fmt.Sprintf("Database error received: %s - %s", errorData.Error, errorData.Message), logger.ERROR)
	} else {
		logger.Log("DBWorker", w.ID, fmt.Sprintf("Database error received: %s", errorData.Error), logger.ERROR)
	}
	
	// Record error metric with specific type
	w.Metrics.RecordError("db_error_received")
	
	// Here you would implement specific error handling for DB operations
	// For example, retrying failed operations, cleaning up resources, etc.
}