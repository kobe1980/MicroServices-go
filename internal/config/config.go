// Package config provides configuration management for the microservices
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	BrokerURL             string `json:"broker_url"`
	Keepalive             int    `json:"keepalive"`
	SystemManagerLog      bool   `json:"SystemManager_log"`
	WorkerLog             bool   `json:"Worker_log"`
	DataTransferProtocol  string `json:"data_transfer_protocol"`
	MetricsPort           int    `json:"metrics_port"`
	JobRetry              int    `json:"job_retry"`
	JobTimeout            int    `json:"job_timeout"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		BrokerURL:             "amqp://localhost",
		Keepalive:             10000,
		SystemManagerLog:      true,
		WorkerLog:             true,
		DataTransferProtocol:  "msgpack",
		MetricsPort:           9091,
		JobRetry:              5,
		JobTimeout:            2000,
	}
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()
	
	// If no config path is specified, use default config
	if configPath == "" {
		configPath = filepath.Join("config", "config.json")
	}
	
	file, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	
	err = json.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}
	
	return config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configPath string) error {
	if configPath == "" {
		configPath = filepath.Join("config", "config.json")
	}
	
	file, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, file, 0644)
}

// GetConfigFromEnv loads configuration from environment variables
// This provides compatibility with the JavaScript version's environment variable support
func GetConfigFromEnv() *Config {
	config := DefaultConfig()
	
	// Read from environment variables, fallback to defaults
	if val := os.Getenv("BROKER_URL"); val != "" {
		config.BrokerURL = val
	}
	
	if val := os.Getenv("DATA_TRANSFER_PROTOCOL"); val != "" {
		config.DataTransferProtocol = val
	}
	
	if val := os.Getenv("METRICS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.MetricsPort = port
		}
	}
	
	if val := os.Getenv("JOB_RETRY"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			config.JobRetry = retries
		}
	}
	
	if val := os.Getenv("JOB_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil {
			config.JobTimeout = timeout
		}
	}
	
	if val := os.Getenv("KEEPALIVE"); val != "" {
		if keepalive, err := strconv.Atoi(val); err == nil {
			config.Keepalive = keepalive
		}
	}
	
	// Boolean flags
	if val := os.Getenv("WORKER_LOG"); val != "" {
		config.WorkerLog = parseBool(val, config.WorkerLog)
	}
	
	if val := os.Getenv("SYSTEM_MANAGER_LOG"); val != "" {
		config.SystemManagerLog = parseBool(val, config.SystemManagerLog)
	}
	
	return config
}

// Helper function to parse boolean environment variables
func parseBool(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes", "y", "Y":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "No", "n", "N":
		return false
	default:
		return defaultValue
	}
}