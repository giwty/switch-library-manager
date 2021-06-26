package logger

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"
)

const (
	LOGGER_FILE = "slm.log"
)

var logger *zap.Logger

// Create new logger
func newLogger(workingFolder string, debug bool) {
	config := zap.NewDevelopmentConfig()

	// If not debug keep at info level
	if !debug {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logPath := filepath.Join(workingFolder, LOGGER_FILE)
	// delete old file
	os.Remove(logPath)

	if runtime.GOOS == "windows" {
		zap.RegisterSink("winfile", func(u *url.URL) (zap.Sink, error) {
			// Remove leading slash left by url.Parse()
			return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		})
		logPath = "winfile:///" + logPath
	}

	config.OutputPaths = []string{logPath}
	config.ErrorOutputPaths = []string{logPath}

	// Creating the logger
	var loggerErr error
	logger, loggerErr = config.Build()
	if loggerErr != nil {
		fmt.Printf("failed to create logger - %v", loggerErr)
		panic(1)
	}
	zap.ReplaceGlobals(logger)
}

// Get sugared logger from logger
func GetSugar(workingFolder string, debug bool) *zap.SugaredLogger {
	if logger == nil {
		newLogger(workingFolder, debug)
	}

	return logger.Sugar()
}

// Sync on defer (call it with defer)
func Defer() {
	if logger != nil {
		logger.Sync()
	}
}
