package examples

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/rabbit"
	"github.com/kobe1980/microservices-go/internal/worker"
)

// This example creates a simple client that sends requests to the DBWorker
func RunDBWorkerExample() {
	fmt.Println("Starting DBWorker Example")

	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading config: %s\n", err)
		return
	}

	// Create a client worker (represents an API or client that would interact with the DBWorker)
	clientWorker, err := worker.NewWorker("client", cfg, false)
	if err != nil {
		fmt.Printf("Error creating client worker: %s\n", err)
		return
	}

	// Create a DBWorker
	dbWorker, err := NewDBWorker(cfg, false)
	if err != nil {
		fmt.Printf("Error creating DB worker: %s\n", err)
		clientWorker.Kill()
		return
	}

	// Set up a channel to receive termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	// Listen for responses from the DBWorker
	clientWorker.RabbitContext.OnReady(func() {
		// Set up a subscriber for error responses
		errorSub, err := clientWorker.RabbitContext.NewSocket(rabbit.SUB)
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

		// Set up a subscriber for DB responses
		responseSub, err := clientWorker.RabbitContext.NewSocket(rabbit.SUB)
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
					fmt.Println("Received response from DB worker:")
					
					// Convert the generic response to a DBResponse
					respJson, _ := json.Marshal(jobData.Data)
					var response DBResponse
					json.Unmarshal(respJson, &response)
					
					// Pretty print the response
					respPretty, _ := json.MarshalIndent(response, "", "  ")
					fmt.Println(string(respPretty))
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
		go runDBRequests(clientWorker, done)
	})

	// Wait for termination signal
	go func() {
		<-sigs
		fmt.Println("\nShutting down...")
		clientWorker.Kill()
		dbWorker.Kill()
		done <- true
	}()

	<-done
	fmt.Println("Example completed")
}

// runDBRequests sends a series of example requests to the DBWorker
func runDBRequests(client *worker.Worker, done chan bool) {
	// Wait a moment for everything to connect
	time.Sleep(1 * time.Second)

	fmt.Println("\n----- Running DB Worker Example Requests -----\n")

	// Example 1: Find all users
	fmt.Println("1. Finding all users...")
	sendDBRequest(client, DBRequest{
		Action: "find",
		Table:  "users",
	})
	time.Sleep(1 * time.Second)

	// Example 2: Find user by ID
	fmt.Println("\n2. Finding user with ID 1...")
	sendDBRequest(client, DBRequest{
		Action: "find",
		Table:  "users",
		Query: map[string]interface{}{
			"id": 1,
		},
	})
	time.Sleep(1 * time.Second)

	// Example 3: Insert a new user
	fmt.Println("\n3. Inserting a new user...")
	sendDBRequest(client, DBRequest{
		Action: "insert",
		Table:  "users",
		Data: map[string]interface{}{
			"id":   3,
			"name": "Bob Johnson",
			"age":  42,
		},
	})
	time.Sleep(1 * time.Second)

	// Example 4: Update a user
	fmt.Println("\n4. Updating user with ID 2...")
	sendDBRequest(client, DBRequest{
		Action: "update",
		Table:  "users",
		Query: map[string]interface{}{
			"id": 2,
		},
		Data: map[string]interface{}{
			"name": "Jane Doe",
			"age":  26,
		},
	})
	time.Sleep(1 * time.Second)

	// Example 5: Delete a user
	fmt.Println("\n5. Deleting user with ID 3...")
	sendDBRequest(client, DBRequest{
		Action: "delete",
		Table:  "users",
		Query: map[string]interface{}{
			"id": 3,
		},
	})
	time.Sleep(1 * time.Second)

	// Example 6: Try to find a user that doesn't exist (to demonstrate error handling)
	fmt.Println("\n6. Finding non-existent user (should error)...")
	sendDBRequest(client, DBRequest{
		Action: "find",
		Table:  "users",
		Query: map[string]interface{}{
			"id": 999,
		},
	})
	time.Sleep(1 * time.Second)

	// Example 7: Try to access a non-existent table (to demonstrate error handling)
	fmt.Println("\n7. Accessing non-existent table (should error)...")
	sendDBRequest(client, DBRequest{
		Action: "find",
		Table:  "non_existent_table",
	})
	time.Sleep(1 * time.Second)

	fmt.Println("\n----- Example requests completed -----")
	fmt.Println("Press Ctrl+C to exit")
}

// sendDBRequest sends a request to the DBWorker
func sendDBRequest(client *worker.Worker, request DBRequest) {
	// Set up the worker list (routing path)
	workersList := []string{"db"}

	// Send the request
	client.SendToNextWorker(
		workersList,
		request,
		0,
		"",
		1,
	)
}