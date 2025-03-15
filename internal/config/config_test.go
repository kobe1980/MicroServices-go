package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BrokerURL != "amqp://localhost" {
		t.Errorf("Expected BrokerURL to be 'amqp://localhost', got '%s'", cfg.BrokerURL)
	}

	if cfg.DataTransferProtocol != "msgpack" {
		t.Errorf("Expected DataTransferProtocol to be 'msgpack', got '%s'", cfg.DataTransferProtocol)
	}

	if cfg.JobRetry != 5 {
		t.Errorf("Expected JobRetry to be 5, got %d", cfg.JobRetry)
	}
}

func TestLoadConfig(t *testing.T) {
	// Test with default config path
	cfg, err := LoadConfig("")
	if err != nil {
		// This might fail if config.json doesn't exist in the default location, which is expected in a test environment
		if !os.IsNotExist(err) {
			t.Errorf("Unexpected error loading default config: %v", err)
		}
	}

	// Test with a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")
	
	// Create a test config file
	configContent := `{
		"broker_url": "amqp://testhost",
		"keepalive": 5000,
		"SystemManager_log": false,
		"Worker_log": false,
		"data_transfer_protocol": "json",
		"metrics_port": 8080,
		"job_retry": 3,
		"job_timeout": 1000
	}`
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	// Load the test config
	testCfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	
	// Verify the loaded config
	if testCfg.BrokerURL != "amqp://testhost" {
		t.Errorf("Expected BrokerURL to be 'amqp://testhost', got '%s'", testCfg.BrokerURL)
	}
	
	if testCfg.DataTransferProtocol != "json" {
		t.Errorf("Expected DataTransferProtocol to be 'json', got '%s'", testCfg.DataTransferProtocol)
	}
	
	if testCfg.JobRetry != 3 {
		t.Errorf("Expected JobRetry to be 3, got %d", testCfg.JobRetry)
	}
}

func TestSaveConfig(t *testing.T) {
	// Create a test config
	cfg := &Config{
		BrokerURL:            "amqp://savetest",
		Keepalive:            7000,
		SystemManagerLog:     true,
		WorkerLog:            true,
		DataTransferProtocol: "bson",
		MetricsPort:          9000,
		JobRetry:             10,
		JobTimeout:           5000,
	}
	
	// Save to a temporary file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "saved_config.json")
	
	err := SaveConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}
	
	// Read the saved config
	loadedCfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}
	
	// Verify the loaded config matches the original
	if loadedCfg.BrokerURL != cfg.BrokerURL {
		t.Errorf("Expected BrokerURL to be '%s', got '%s'", cfg.BrokerURL, loadedCfg.BrokerURL)
	}
	
	if loadedCfg.DataTransferProtocol != cfg.DataTransferProtocol {
		t.Errorf("Expected DataTransferProtocol to be '%s', got '%s'", 
			cfg.DataTransferProtocol, loadedCfg.DataTransferProtocol)
	}
	
	if loadedCfg.JobRetry != cfg.JobRetry {
		t.Errorf("Expected JobRetry to be %d, got %d", cfg.JobRetry, loadedCfg.JobRetry)
	}
}