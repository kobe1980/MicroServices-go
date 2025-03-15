# MicroServices-go

A microservices architecture ready to use, built in Go. This is a Go implementation of the original [JavaScript MicroServices project](https://github.com/kobe1980/MicroServices).

Every service communicates through the RabbitMQ broker.

## Dependencies

- [amqp091-go](https://github.com/rabbitmq/amqp091-go) - Modern RabbitMQ client library. RabbitMQ is not included; you need to install it separately.
- [msgpack](https://github.com/vmihailenco/msgpack) - MessagePack encoding for Golang
- [go.mongodb.org/mongo-driver](https://github.com/mongodb/mongo-go-driver) - BSON implementation for efficient binary message serialization
- [uuid](https://github.com/google/uuid) - Used for generating unique IDs
- [prometheus/client_golang](https://github.com/prometheus/client_golang) - Prometheus client for metrics collection and monitoring

## Architecture Overview

![Architecture Overview](docs/archi.png)

## Components

1. SystemManager - Central coordinator that tracks available workers
2. RabbitMQ Bus - Message broker for communication between components
3. Workers - Specialized service components that perform specific tasks
4. Metrics - Monitoring system for collecting and exposing performance data

## Global Behavior

The SystemManager starts and waits for workers to register themselves on the bus. Each worker connects to the bus and announces itself with its configuration.

When a client wants to use a service, it sends a message to the first worker in the worker list for the job it wants to execute. If the worker is available, it processes the job and responds directly to the client. If the worker is busy, it passes the request to the next worker in the list.

If no worker is available, an error is sent to the client. If a worker is taking too long to respond, the client will resend the job a configurable number of times before considering it failed.

## Installation

```bash
# Clone the repository
git clone https://github.com/kobe1980/microservices-go.git

# Change directory
cd microservices-go

# Build
go build -o bin/ ./...
```

## Usage

Start the SystemManager:

```bash
./bin/systemmanager
```

Start one or more workers:

```bash
./bin/worker
```

### Running Examples

The project includes example implementations to demonstrate how to use the framework:

#### DBWorker Example

The DBWorker example shows how to create a specialized worker that handles database operations. It demonstrates:

- Creating a custom worker that extends the base Worker
- Handling different types of operations (find, insert, update, delete)
- Processing requests and sending responses
- Error handling

To run the DBWorker example:

```bash
# Make sure RabbitMQ is running first
go run cmd/examples/main.go -type db
```

#### RESTWorker Example

The RESTWorker example demonstrates how to create a worker that makes HTTP requests to external APIs. It supports:

- All common HTTP methods (GET, POST, PUT, DELETE, PATCH)
- Custom headers
- JSON request bodies
- JSON response handling
- Error handling for failed requests

To run the RESTWorker example:

```bash
# Make sure RabbitMQ is running first
go run cmd/examples/main.go -type rest
```

### Creating Your Own Workers

To create a custom worker:

1. Create a new struct that embeds the base Worker:
   ```go
   type MyWorker struct {
       *worker.Worker
       // Add your custom fields here
   }
   ```

2. Create a constructor function:
   ```go
   func NewMyWorker(cfg *config.Config, metricsDisabled bool) (*MyWorker, error) {
       baseWorker, err := worker.NewWorker("my-type", cfg, metricsDisabled)
       if err != nil {
           return nil, err
       }
       
       return &MyWorker{
           Worker: baseWorker,
           // Initialize your custom fields
       }, nil
   }
   ```

3. Override at least the `DoJob` and `HandleError` methods:
   ```go
   // Override DoJob to handle your worker's specific job processing
   func (w *MyWorker) DoJob(jobData worker.JobData) {
       // Your implementation here
       
       // When done, release the job lock
       w.SetNextJobForMe(false)
       
       // Stop the metric timer
       if jobData.MetricTimer != nil {
           jobData.MetricTimer()
       }
   }
   
   // Override HandleError to handle your worker's specific error handling
   func (w *MyWorker) HandleError(errorData worker.Error) {
       // Your implementation here
   }
   ```

4. Optionally override the `GetConfig` method to provide additional task information:
   ```go
   func (w *MyWorker) GetConfig() worker.WorkerConfig {
       config := w.Worker.GetConfig()
       config.Tasks = []string{"task1", "task2"}
       return config
   }
   ```

See `internal/examples/dbworker.go` for a complete example.

## Tests

Tests are built with the Go testing package and testify for assertions.

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## License

MIT

## Version

0.0.1 - Initial Go implementation