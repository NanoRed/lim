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

// Info info log
func Info(format string, v ...interface{}) {
	logger.Info(1, format, v...)
}

// Warn warning log
func Warn(format string, v ...interface{}) {
	logger.Warn(1, format, v...)
}

// Error error log
func Error(format string, v ...interface{}) {
	logger.Error(1, format, v...)
}

// Panic panic log and cause a panic
func Panic(format string, v ...interface{}) {
	logger.Panic(1, format, v...)
}

// Fatal fatal log and exit
func Fatal(format string, v ...interface{}) {
	logger.Fatal(1, format, v...)
}
