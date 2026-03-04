package logging

import (
	"fmt"
	"time"
)

var _ ILogger = (*DefaultLogger)(nil)

var GlobalLogDataBuilder func(data *LogData)

type DefaultLogger struct {
	path           string
	handler        ILogHandler
	logDataBuilder func(data *LogData)
	filter         ILogFilter
}

func NewDefaultLogger(path string, handler ILogHandler, logDataBuilder func(data *LogData)) *DefaultLogger {
	return &DefaultLogger{
		path:           path,
		handler:        handler,
		logDataBuilder: logDataBuilder,
	}
}

func (ss *DefaultLogger) SetFilter(filter ILogFilter) {
	ss.filter = filter
}

func (ss *DefaultLogger) logf(level Level, format string, args ...any) {
	if ss.filter != nil && !ss.filter.ShouldLog(level, "", ss.path) {
		return
	}
	logData := &LogData{
		Time:   time.Now(),
		Path:   ss.path,
		Level:  level,
		Custom: args,
		Message: func() string {
			return fmt.Sprintf(format, args...)
		},
	}
	logData.ErrorCode = ExtractErrorCode(args)
	if GlobalLogDataBuilder != nil {
		GlobalLogDataBuilder(logData)
	}
	if ss.logDataBuilder != nil {
		ss.logDataBuilder(logData)
	}
	if ss.handler != nil {
		ss.handler.Log(logData)
	}
}

func (ss *DefaultLogger) Tracef(format string, args ...any) {
	ss.logf(TRACE, format, args...)
}

func (ss *DefaultLogger) Debugf(format string, args ...any) {
	ss.logf(DEBUG, format, args...)
}

func (ss *DefaultLogger) Infof(format string, args ...any) {
	ss.logf(INFO, format, args...)
}

func (ss *DefaultLogger) Warnf(format string, args ...any) {
	ss.logf(WARN, format, args...)
}

func (ss *DefaultLogger) Errorf(format string, args ...any) {
	ss.logf(ERROR, format, args...)
}

func (ss *DefaultLogger) Fatalf(format string, args ...any) {
	ss.logf(FATAL, format, args...)
}
