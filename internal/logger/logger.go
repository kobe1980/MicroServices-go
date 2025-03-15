// Package logger provides structured logging services for the microservices
package logger

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	// Set up logrus
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
}

// LogLevel represents the severity of a log message
type LogLevel string

const (
	// INFO level for general informational messages
	INFO LogLevel = "INFO"
	// WARN level for warning messages
	WARN LogLevel = "WARN"
	// ERROR level for error messages
	ERROR LogLevel = "ERROR"
)

// Log logs a message with the specified service, function, message, and level
func Log(service string, function string, message string, level ...LogLevel) {
	// Default to INFO level if not specified
	logLevel := INFO
	if len(level) > 0 {
		logLevel = level[0]
	}

	// Get current timestamp
	timestamp := time.Now().Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	
	// Format log message similar to the original MicroServices project
	formattedMsg := fmt.Sprintf("%s - [32mMicroService.%s - %s[39m => %s", 
		timestamp, service, function, message)
	
	// Log with appropriate level
	switch logLevel {
	case ERROR:
		formattedMsg = fmt.Sprintf("%s - [31mMicroService.%s - %s[39m => %s", 
			timestamp, service, function, message)
		log.Error(formattedMsg)
	default:
		log.Info(formattedMsg)
	}
}

// SetLevel sets the global log level
func SetLevel(level logrus.Level) {
	log.SetLevel(level)
}

// GetLogger returns the underlying logger
func GetLogger() *logrus.Logger {
	return log
}