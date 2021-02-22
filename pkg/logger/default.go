package logger

import (
	"fmt"
	"log"
	"os"
)

// DefaultLogger default logger
type DefaultLogger struct {
	info    *log.Logger
	warning *log.Logger
	err     *log.Logger
	panic   *log.Logger
	fatal   *log.Logger
}

// NewDefaultLogger create a default logger
func NewDefaultLogger(logger ...*log.Logger) *DefaultLogger {
	return &DefaultLogger{logger[0], logger[1], logger[2], logger[3], logger[4]}
}

// Info info log
func (l *DefaultLogger) Info(stackoffset int, format string, v ...interface{}) {
	l.info.Output(2+int(stackoffset), fmt.Sprintf(format, v...))
}

// Warn warning log
func (l *DefaultLogger) Warn(stackoffset int, format string, v ...interface{}) {
	l.warning.Output(2+int(stackoffset), fmt.Sprintf(format, v...))
}

// Error error log
func (l *DefaultLogger) Error(stackoffset int, format string, v ...interface{}) {
	l.err.Output(2+int(stackoffset), fmt.Sprintf(format, v...))
}

// Panic panic log and cause a panic
func (l *DefaultLogger) Panic(stackoffset int, format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.panic.Output(2+int(stackoffset), s)
	panic(s)
}

// Fatal fatal log and exit
func (l *DefaultLogger) Fatal(stackoffset int, format string, v ...interface{}) {
	l.fatal.Output(2+int(stackoffset), fmt.Sprintf(format, v...))
	os.Exit(1)
}
