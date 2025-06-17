package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func Init() {
	Log = logrus.New()

	// Configure output format
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set output to stdout
	Log.SetOutput(os.Stdout)

	// Set log level based on environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}

	Log.Info("Logger initialized successfully")
}

// Utility functions for common logging patterns
func LogError(operation string, err error, fields map[string]interface{}) {
	entry := Log.WithFields(logrus.Fields{
		"operation": operation,
		"error":     err.Error(),
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Error("Operation failed")
}

func LogInfo(operation string, message string, fields map[string]interface{}) {
	entry := Log.WithFields(logrus.Fields{
		"operation": operation,
		"message":   message,
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Info(message)
}

func LogDebug(operation string, message string, fields map[string]interface{}) {
	entry := Log.WithFields(logrus.Fields{
		"operation": operation,
		"message":   message,
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Debug(message)
}

func LogWarn(operation string, message string, fields map[string]interface{}) {
	entry := Log.WithFields(logrus.Fields{
		"operation": operation,
		"message":   message,
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Warn(message)
}
