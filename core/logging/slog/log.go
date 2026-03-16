package slog

import (
	"sync/atomic"

	"github.com/gmbytes/snow/core/logging"
)

var globalLogger atomic.Pointer[logging.ILogger]

func init() {
	logger := logging.ILogger(logging.NewDefaultLogger("Global", logging.NewSimpleLogHandler(), nil))
	globalLogger.Store(&logger)
}

func BindGlobalHandler(h logging.ILogHandler) {
	logger := logging.ILogger(logging.NewDefaultLogger("Global", h, nil))
	globalLogger.Store(&logger)
}

func BindGlobalLogger(l logging.ILogger) {
	if l == nil {
		panic("bind global logger with nil")
	}

	globalLogger.Store(&l)
}

func getLogger() logging.ILogger {
	logger := globalLogger.Load()
	if logger == nil {
		// 返回默认 logger，避免 nil pointer
		defaultLogger := logging.ILogger(logging.NewDefaultLogger("Global", logging.NewSimpleLogHandler(), nil))
		globalLogger.Store(&defaultLogger)
		return defaultLogger
	}
	return *logger
}

func Tracef(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Tracef(format, args...)
	}
}

func Debugf(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Debugf(format, args...)
	}
}

func Infof(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Infof(format, args...)
	}
}

func Warnf(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Warnf(format, args...)
	}
}

func Errorf(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Errorf(format, args...)
	}
}

func Fatalf(format string, args ...any) {
	if logger := getLogger(); logger != nil {
		logger.Fatalf(format, args...)
	}
}
