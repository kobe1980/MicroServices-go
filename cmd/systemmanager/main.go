package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/kobe1980/microservices-go/internal/systemmanager"
	"github.com/sirupsen/logrus"
)

var (
	configFile      string
	verbose         bool
	disableMetrics  bool
)

func init() {
	flag.StringVar(&configFile, "config", "", "Path to config file")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&disableMetrics, "disable-metrics", false, "Disable metrics collection")
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Set log level based on verbosity
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create system manager
	manager, err := systemmanager.NewSystemManager(cfg, disableMetrics)
	if err != nil {
		fmt.Printf("Failed to create system manager: %v\n", err)
		os.Exit(1)
	}

	// Wait for termination signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	fmt.Println("Shutting down...")
	
	// Clean up
	manager.Kill()
}