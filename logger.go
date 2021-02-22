package lim

import (
	"fmt"
	"log"
	"os"
)

// Logger application logger
type Logger interface {
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Panic(format string, v ...interface{})
	Fatal(format string, v ...interface{})
}

var logger Logger

func init() {
	flag := log.LstdFlags | log.Lmicroseconds | log.Lshortfile | log.Lmsgprefix
	logger = NewDefaultLogger(
		log.New(os.Stdout, "INFO: ", flag),
		log.New(os.Stdout, "WARNING: ", flag),
		log.New(os.Stdout, "ERROR: ", flag),
		log.New(os.Stdout, "PANIC: ", flag),
		log.New(os.Stdout, "FATAL: ", flag),
	)
}

// RegisterLogger register a logger
func RegisterLogger(l Logger) {
	logger = l
}

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
func (l *DefaultLogger) Info(format string, v ...interface{}) {
	l.info.Output(2, fmt.Sprintf(format, v...))
}

// Warn warning log
func (l *DefaultLogger) Warn(format string, v ...interface{}) {
	l.warning.Output(2, fmt.Sprintf(format, v...))
}

// Error error log
func (l *DefaultLogger) Error(format string, v ...interface{}) {
	l.err.Output(2, fmt.Sprintf(format, v...))
}

// Panic panic log and cause a panic
func (l *DefaultLogger) Panic(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.panic.Output(2, s)
	panic(s)
}

// Fatal fatal log and exit
func (l *DefaultLogger) Fatal(format string, v ...interface{}) {
	l.fatal.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}
