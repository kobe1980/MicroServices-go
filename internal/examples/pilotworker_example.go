package examples

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/pilot"
)

// RunPilotWorkerExample demonstrates how to use the PilotWorker
func RunPilotWorkerExample() {
	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading config: %s\n", err)
		return
	}

	// Enable worker logging
	cfg.WorkerLog = true

	// Create a new pilot worker
	worker, err := pilot.NewPilotWorker(cfg, false)
	if err != nil {
		fmt.Printf("Error creating pilot worker: %s\n", err)
		return
	}

	// Print worker information
	fmt.Printf("PilotWorker started with ID: %s\n", worker.ID)
	fmt.Printf("PilotWorker type: %s\n", worker.Type)
	
	// Create a channel for shutdown coordination
	shutdown := make(chan struct{})
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Handle signals in a separate goroutine
	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal %v, shutting down PilotWorker...\n", sig)
		worker.Kill()
		close(shutdown)
	}()

	// Create a ticker for sending test pipelines (every 30 seconds)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Println("PilotWorker is running. Press Ctrl+C to exit.")

	// Example of how to create and send a pipeline request programmatically
	fmt.Println("\nExample pipeline request:")
	pipelineExample := map[string]interface{}{
		"pipeline": []string{"rest", "db", "rest"},
		"data": map[string]interface{}{
			"method": "GET",
			"url":    "https://api.example.com/data",
		},
		"options": map[string]interface{}{
			"timeout": 5000.0, // 5 seconds in milliseconds
		},
		"requestId": fmt.Sprintf("example-req-%d", time.Now().Unix()),
	}

	fmt.Printf("%+v\n", pipelineExample)
	fmt.Println("\nWaiting for jobs...")

	// Run until terminated
	tickerLoop:
	for {
		select {
		case <-shutdown:
			break tickerLoop
		case <-ticker.C:
			// Log some statistics periodically
			logger.Log("PilotWorkerExample", worker.ID, fmt.Sprintf(
				"PilotWorker has %d pending jobs",
				len(worker.PendingJobs),
			))
		}
	}
	
	fmt.Println("PilotWorker has been shut down")
}