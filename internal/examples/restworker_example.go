package examples

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// This example creates a simple client that sends requests to the RESTWorker
func RunRESTWorkerExample() {
	fmt.Println("Starting RESTWorker Example")

	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading config: %s\n", err)
		return
	}

	// Create a client worker (represents an API or client that would interact with the RESTWorker)
	clientWorker, err := worker.NewWorker("client", cfg, false)
	if err != nil {
		fmt.Printf("Error creating client worker: %s\n", err)
		return
	}

	// Create a RESTWorker
	restWorker, err := NewRESTWorker(cfg, false)
	if err != nil {
		fmt.Printf("Error creating REST worker: %s\n", err)
		clientWorker.Kill()
		return
	}

	// Set up a channel to receive termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	// Listen for responses from the RESTWorker
	clientWorker.RabbitContext.OnReady(func() {
		// Set up a subscriber for error responses
		errorSub, err := clientWorker.RabbitContext.NewSocket(clientWorker.RabbitContext.SocketType("SUB"))
		if err != nil {
			fmt.Printf("Error creating error socket: %s\n", err)
			return
		}

		// Connect to error topic
		err = errorSub.Connect("notifications", "error", func() {
			fmt.Println("Client connected to error notifications")

			// Handle error messages
			err := errorSub.On("data", func(data []byte) {
				var errorData worker.Error
				if err := clientWorker.Compressor.Deserialize(data, &errorData); err != nil {
					fmt.Printf("Error deserializing error response: %s\n", err)
					return
				}

				// Only handle errors targeted at this client
				if errorData.Target.ID == clientWorker.ID {
					fmt.Printf("Received error: %s\n", errorData.Error)
					errDataJson, _ := json.MarshalIndent(errorData, "", "  ")
					fmt.Printf("Error data: %s\n", string(errDataJson))
				}
			})
			if err != nil {
				fmt.Printf("Error setting up error handler: %s\n", err)
			}
		})
		if err != nil {
			fmt.Printf("Error connecting to error notifications: %s\n", err)
		}

		// Set up a subscriber for REST responses
		responseSub, err := clientWorker.RabbitContext.NewSocket(clientWorker.RabbitContext.SocketType("SUB"))
		if err != nil {
			fmt.Printf("Error creating response socket: %s\n", err)
			return
		}

		// Connect to worker.next topic (to receive responses)
		err = responseSub.Connect("notifications", "worker.next", func() {
			fmt.Println("Client connected to worker.next notifications")

			// Handle response messages
			err := responseSub.On("data", func(data []byte) {
				var jobData worker.JobData
				if err := clientWorker.Compressor.Deserialize(data, &jobData); err != nil {
					fmt.Printf("Error deserializing job response: %s\n", err)
					return
				}

				// Only handle responses for this client
				if jobData.WorkersList[jobData.WorkersListID] == "client" {
					fmt.Println("Received response from REST worker:")
					
					// Convert the generic response to a RESTResponse
					respJson, _ := json.Marshal(jobData.Data)
					var response RESTResponse
					json.Unmarshal(respJson, &response)
					
					// Pretty print the response
					fmt.Printf("Status: %d %s\n", response.Status, response.StatusText)
					
					// Print headers
					if len(response.Headers) > 0 {
						fmt.Println("Headers:")
						for k, v := range response.Headers {
							fmt.Printf("  %s: %s\n", k, v)
						}
					}
					
					// Print body (if it's a map or slice, pretty print it)
					fmt.Println("Body:")
					switch v := response.Body.(type) {
					case map[string]interface{}, []interface{}:
						bodyJson, _ := json.MarshalIndent(v, "  ", "  ")
						fmt.Println(string(bodyJson))
					default:
						fmt.Printf("  %v\n", response.Body)
					}
				}
			})
			if err != nil {
				fmt.Printf("Error setting up response handler: %s\n", err)
			}
		})
		if err != nil {
			fmt.Printf("Error connecting to response notifications: %s\n", err)
		}

		// Give workers time to initialize
		time.Sleep(1 * time.Second)

		// Send some example requests
		go runRESTRequests(clientWorker, done)
	})

	// Wait for termination signal
	go func() {
		<-sigs
		fmt.Println("\nShutting down...")
		clientWorker.Kill()
		restWorker.Kill()
		done <- true
	}()

	<-done
	fmt.Println("Example completed")
}

// runRESTRequests sends a series of example requests to the RESTWorker
func runRESTRequests(client *worker.Worker, done chan bool) {
	// Wait a moment for everything to connect
	time.Sleep(1 * time.Second)

	fmt.Println("\n----- Running REST Worker Example Requests -----\n")

	// Example 1: GET request to a public API
	fmt.Println("1. Making GET request to JSONPlaceholder API...")
	sendRESTRequest(client, RESTRequest{
		Method: "GET",
		URL:    "https://jsonplaceholder.typicode.com/posts/1",
	})
	time.Sleep(2 * time.Second)

	// Example 2: GET request with headers
	fmt.Println("\n2. Making GET request with custom headers...")
	sendRESTRequest(client, RESTRequest{
		Method: "GET",
		URL:    "https://httpbin.org/headers",
		Headers: map[string]string{
			"X-Custom-Header": "example-value",
			"Accept":          "application/json",
		},
	})
	time.Sleep(2 * time.Second)

	// Example 3: POST request with JSON body
	fmt.Println("\n3. Making POST request with JSON body...")
	sendRESTRequest(client, RESTRequest{
		Method: "POST",
		URL:    "https://httpbin.org/post",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		},
	})
	time.Sleep(2 * time.Second)

	// Example 4: PUT request
	fmt.Println("\n4. Making PUT request...")
	sendRESTRequest(client, RESTRequest{
		Method: "PUT",
		URL:    "https://httpbin.org/put",
		Body: map[string]interface{}{
			"id":    1,
			"name":  "Jane Smith",
			"email": "jane@example.com",
		},
	})
	time.Sleep(2 * time.Second)

	// Example 5: DELETE request
	fmt.Println("\n5. Making DELETE request...")
	sendRESTRequest(client, RESTRequest{
		Method: "DELETE",
		URL:    "https://httpbin.org/delete",
		Headers: map[string]string{
			"Authorization": "Bearer example-token",
		},
	})
	time.Sleep(2 * time.Second)

	// Example 6: Request to non-existent URL (to demonstrate error handling)
	fmt.Println("\n6. Making request to non-existent URL (should error)...")
	sendRESTRequest(client, RESTRequest{
		Method: "GET",
		URL:    "https://this-domain-does-not-exist.example",
	})
	time.Sleep(2 * time.Second)

	fmt.Println("\n----- Example requests completed -----")
	fmt.Println("Press Ctrl+C to exit")
}

// sendRESTRequest sends a request to the RESTWorker
func sendRESTRequest(client *worker.Worker, request RESTRequest) {
	// Set up the worker list (routing path)
	workersList := []string{"rest"}

	// Send the request
	client.SendToNextWorker(
		workersList,
		request,
		0,
		"",
		1,
	)
}