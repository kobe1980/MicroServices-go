// Example runner for the microservices framework
// Demonstrates how to use the different worker types
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kobe1980/microservices-go/internal/examples"
)

func main() {
	// Define command line flags
	exampleType := flag.String("type", "", "Type of example to run: db, rest")
	
	// Parse flags
	flag.Parse()
	
	// Check if example type was provided
	if *exampleType == "" {
		fmt.Println("Please specify the type of example to run using the -type flag")
		fmt.Println("Available examples: db, rest")
		os.Exit(1)
	}
	
	// Run the appropriate example
	switch *exampleType {
	case "db":
		examples.RunDBWorkerExample()
	case "rest":
		examples.RunRESTWorkerExample()
	default:
		fmt.Printf("Unknown example type: %s\n", *exampleType)
		fmt.Println("Available examples: db, rest")
		os.Exit(1)
	}
}