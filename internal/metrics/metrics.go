// Package metrics provides Prometheus-based metrics collection for monitoring
package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kobe1980/microservices-go/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics struct holds all the metrics collectors
type Metrics struct {
	// Counter metrics
	messagesReceived *prometheus.CounterVec
	messagesSent     *prometheus.CounterVec
	jobErrorsTotal   *prometheus.CounterVec

	// Gauge metrics
	workersCount     *prometheus.GaugeVec
	connectedWorkers prometheus.Gauge

	// Histogram metrics
	jobProcessingTime *prometheus.HistogramVec

	// Service name
	serviceName string

	// HTTP server
	server *http.Server

	// Registry
	registry *prometheus.Registry
}

// Initialize a new metrics collector
func InitMetrics(serviceName string, port int, disableHTTP bool) *Metrics {
	// Create a new registry
	registry := prometheus.NewRegistry()

	// Add the Go collector to the registry
	registry.MustRegister(prometheus.NewGoCollector())

	// Create counter metrics
	messagesReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "microservices_messages_received_total",
			Help: "Total number of messages received by service",
		},
		[]string{"service", "type"},
	)

	messagesSent := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "microservices_messages_sent_total",
			Help: "Total number of messages sent by service",
		},
		[]string{"service", "type"},
	)

	jobErrorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "microservices_job_errors_total",
			Help: "Total number of job processing errors",
		},
		[]string{"service", "error_type"},
	)

	// Create gauge metrics
	workersCount := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "microservices_workers_total",
			Help: "Number of workers by type",
		},
		[]string{"type"},
	)

	connectedWorkers := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "microservices_connected_workers",
			Help: "Number of workers connected to the bus",
		},
	)

	// Create histogram metrics
	jobProcessingTime := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "microservices_job_processing_seconds",
			Help:    "Time spent processing jobs",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		},
		[]string{"service", "job_type"},
	)

	// Register all metrics
	registry.MustRegister(messagesReceived, messagesSent, jobErrorsTotal)
	registry.MustRegister(workersCount, connectedWorkers)
	registry.MustRegister(jobProcessingTime)

	metrics := &Metrics{
		messagesReceived:  messagesReceived,
		messagesSent:      messagesSent,
		jobErrorsTotal:    jobErrorsTotal,
		workersCount:      workersCount,
		connectedWorkers:  connectedWorkers,
		jobProcessingTime: jobProcessingTime,
		serviceName:       serviceName,
		registry:          registry,
	}

	// Start HTTP server if not disabled
	if !disableHTTP {
		// Use the specified port or default to 9091
		if port == 0 {
			port = 9091
		}

		// Create a new HTTP server
		metrics.server = &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		}

		// Start the HTTP server in a new goroutine
		go func() {
			logger.Log("MicroService", "Metrics", 
				fmt.Sprintf("Metrics server started on port %d", port))
			if err := metrics.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Log("MicroService", "Metrics", 
					fmt.Sprintf("Error starting metrics server: %s", err.Error()), logger.ERROR)
			}
		}()
	}

	return metrics
}

// RecordMessageReceived increments the counter for received messages
func (m *Metrics) RecordMessageReceived(messageType string) {
	m.messagesReceived.WithLabelValues(m.serviceName, messageType).Inc()
}

// RecordMessageSent increments the counter for sent messages
func (m *Metrics) RecordMessageSent(messageType string) {
	m.messagesSent.WithLabelValues(m.serviceName, messageType).Inc()
}

// StartJobTimer returns a function that, when called, records the duration of a job
func (m *Metrics) StartJobTimer(jobType string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start).Seconds()
		m.jobProcessingTime.WithLabelValues(m.serviceName, jobType).Observe(duration)
	}
}

// SetWorkerCount sets the gauge for the number of workers by type
func (m *Metrics) SetWorkerCount(workerType string, count int) {
	m.workersCount.WithLabelValues(workerType).Set(float64(count))
}

// RecordError increments the counter for errors
func (m *Metrics) RecordError(errorType string) {
	m.jobErrorsTotal.WithLabelValues(m.serviceName, errorType).Inc()
}

// SetConnectedWorkers sets the gauge for the total number of connected workers
func (m *Metrics) SetConnectedWorkers(count int) {
	m.connectedWorkers.Set(float64(count))
}

// Close shuts down the metrics HTTP server
func (m *Metrics) Close() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}