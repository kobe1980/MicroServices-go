package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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