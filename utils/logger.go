package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	ErrorLogger *log.Logger
	PanicLogger *log.Logger
)

func InitLogger() error {
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	errorLogFile, err := os.OpenFile(filepath.Join(logsDir, "errors.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %v", err)
	}

	panicLogFile, err := os.OpenFile(filepath.Join(logsDir, "panics.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open panic log file: %v", err)
	}

	ErrorLogger = log.New(errorLogFile, "", 0)
	PanicLogger = log.New(panicLogFile, "", 0)

	return nil
}

func LogError(err error, context string) {
	if ErrorLogger == nil {
		return
	}

	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	ErrorLogger.Printf("[%s] ERROR in %s:%d - %s: %v", timestamp, filepath.Base(file), line, context, err)
}

func LogPanic(recovered interface{}, context string) {
	if PanicLogger == nil {
		return
	}

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	PanicLogger.Printf("[%s] PANIC in %s:%d - %s: %v", timestamp, filepath.Base(file), line, context, recovered)
}
