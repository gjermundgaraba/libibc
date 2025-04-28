package logging

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const logFolder = "logs"

var _ zapcore.WriteSyncer = &IBCLogWriter{}

type IBCLogWriter struct {
	mutex        sync.Mutex
	logFile      *os.File
	extraLoggers []func(string)
}

func NewIBCLogger(logLevel string) (*zap.Logger, *IBCLogWriter) {
	numFilesInFolder := 0
	files, err := os.ReadDir(logFolder)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			numFilesInFolder++
		}
	}
	nextNum := numFilesInFolder + 1

	logFileName := fmt.Sprintf("ibc-%d-%d.log", nextNum, time.Now().Unix())
	logPath := fmt.Sprintf("%s/%s", logFolder, logFileName)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	level := getLogLevel(logLevel)
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logWriter := &IBCLogWriter{
		mutex:        sync.Mutex{},
		logFile:      logFile,
		extraLoggers: make([]func(string), 0),
	}

	zapCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(logWriter),
		level,
	)

	logger := zap.New(zapCore)

	return logger, logWriter
}

func (w *IBCLogWriter) AddExtraLogger(logger func(string)) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.extraLoggers = append(w.extraLoggers, logger)
}

// Write implements io.Writer interface
func (w *IBCLogWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Add log entries for extra loggers
	for _, logger := range w.extraLoggers {
		logger(string(p))
	}

	// Write to the log file
	_, err = w.logFile.Write(p)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Sync implements zapcore.WriteSyncer interface
func (w *IBCLogWriter) Sync() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	return w.logFile.Sync()
}

// SetLogLevel sets the log level from a string
func getLogLevel(logLevel string) zap.AtomicLevel {
	switch logLevel {
	case "debug":
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		return zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		panic("invalid log level")
	}
}
