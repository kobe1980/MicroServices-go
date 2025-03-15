// Package logger provides structured logging for the microservices
package logger

import (
	"bytes"
	"testing"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// The tests in this file verify the behavior of the logger package, including:
// - Basic logging at different levels (INFO, ERROR)
// - Setting log levels
// - Getting the underlying logger instance

func TestLog(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	origLogger := log
	defer func() { log = origLogger }() // Restore original logger

	// Create a test logger
	testLogger := logrus.New()
	testLogger.SetOutput(&buf)
	testLogger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
	log = testLogger

	// Test INFO level log
	buf.Reset()
	Log("TestService", "TestFunction", "Test message")
	output := buf.String()
	assert.Contains(t, output, "level=info")
	assert.Contains(t, output, "MicroService.TestService - TestFunction")
	assert.Contains(t, output, "Test message")

	// Test ERROR level log
	buf.Reset()
	Log("TestService", "TestFunction", "Error message", ERROR)
	output = buf.String()
	assert.Contains(t, output, "level=error")
	assert.Contains(t, output, "MicroService.TestService - TestFunction")
	assert.Contains(t, output, "Error message")
}

func TestSetLevel(t *testing.T) {
	// Capture current level
	origLevel := log.GetLevel()
	defer func() { log.SetLevel(origLevel) }() // Restore original level

	// Set to DEBUG level
	SetLevel(logrus.DebugLevel)
	assert.Equal(t, logrus.DebugLevel, log.GetLevel())

	// Set to ERROR level
	SetLevel(logrus.ErrorLevel)
	assert.Equal(t, logrus.ErrorLevel, log.GetLevel())
}

func TestGetLogger(t *testing.T) {
	// GetLogger should return the current logger
	logger := GetLogger()
	assert.Equal(t, log, logger)
}