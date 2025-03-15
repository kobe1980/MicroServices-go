package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kobe1980/microservices-go/internal/examples"
)

func main() {
	fmt.Println("Starting PilotWorker example...")
	
	// Create a channel to signal when the example is done
	done := make(chan struct{})
	
	// Run the example in a goroutine
	go func() {
		examples.RunPilotWorkerExample()
		close(done)
	}()
	
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Wait for either completion or signal
	select {
	case <-done:
		// Example completed normally
		fmt.Println("PilotWorker example completed")
	case sig := <-sigChan:
		// Signal received
		fmt.Printf("\nReceived signal %v, forcing exit...\n", sig)
		time.Sleep(500 * time.Millisecond) // Allow some time for cleanup
		os.Exit(0)
	}
}