// Package metrics provides Prometheus-based metrics collection for the microservices
package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// The tests in this file verify the behavior of the metrics package, including:
// - Initializing metrics collectors
// - Recording various types of metrics (counters, gauges, histograms)
// - Starting and stopping timers
// - HTTP server setup for metrics exposure
// - Proper cleanup of resources

func TestInitMetrics(t *testing.T) {
	// Initialize metrics with HTTP disabled to avoid port conflicts in tests
	m := InitMetrics("test-service", 0, true)
	defer m.Close()
	
	// Verify metrics were initialized
	assert.NotNil(t, m)
	assert.Equal(t, "test-service", m.serviceName)
	assert.NotNil(t, m.messagesReceived)
	assert.NotNil(t, m.messagesSent)
	assert.NotNil(t, m.jobErrorsTotal)
	assert.NotNil(t, m.workersCount)
	assert.NotNil(t, m.connectedWorkers)
	assert.NotNil(t, m.jobProcessingTime)
	assert.NotNil(t, m.registry)
	assert.Nil(t, m.server) // Server should be nil with HTTP disabled
}

func TestInitMetricsWithHTTP(t *testing.T) {
	// Use a high port number to avoid conflicts
	port := 29999
	m := InitMetrics("test-service", port, false)
	defer m.Close()
	
	// Verify HTTP server was initialized
	assert.NotNil(t, m.server)
	assert.Equal(t, ":29999", m.server.Addr)
}

func TestRecordMetrics(t *testing.T) {
	// Initialize metrics with HTTP disabled
	m := InitMetrics("test-service", 0, true)
	defer m.Close()
	
	// Test recording metrics (should not panic)
	
	// Record message received
	m.RecordMessageReceived("test-message")
	
	// Record message sent
	m.RecordMessageSent("test-message")
	
	// Record error
	m.RecordError("test-error")
	
	// Set worker count
	m.SetWorkerCount("test-worker", 5)
	
	// Set connected workers
	m.SetConnectedWorkers(10)
}

func TestJobTimer(t *testing.T) {
	// Initialize metrics with HTTP disabled
	m := InitMetrics("test-service", 0, true)
	defer m.Close()
	
	// Start a job timer
	stop := m.StartJobTimer("test-job")
	assert.NotNil(t, stop)
	
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	
	// Stop the timer (should not panic)
	stop()
}

func TestClose(t *testing.T) {
	// Initialize metrics with HTTP disabled
	m := InitMetrics("test-service", 0, true)
	
	// Close should not error
	err := m.Close()
	assert.NoError(t, err)
	
	// Close with HTTP server
	port := 29998
	m2 := InitMetrics("test-service", port, false)
	
	err = m2.Close()
	assert.NoError(t, err)
}

func TestCollectors(t *testing.T) {
	// Test that all collectors are registered properly
	registry := prometheus.NewRegistry()
	
	// Create counter metrics
	messagesReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_messages_received_total",
			Help: "Total number of messages received by service",
		},
		[]string{"service", "type"},
	)
	
	messagesSent := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_messages_sent_total",
			Help: "Total number of messages sent by service",
		},
		[]string{"service", "type"},
	)
	
	// Register metrics
	err := registry.Register(messagesReceived)
	assert.NoError(t, err)
	
	err = registry.Register(messagesSent)
	assert.NoError(t, err)
	
	// Test using the collectors
	messagesReceived.WithLabelValues("test", "message").Inc()
	messagesSent.WithLabelValues("test", "message").Inc()
}