package logger

import (
	"fmt"
	"log"
	"os"
)

// Logger application logger
// stackoffset is used to locate the position of the log in the calling stack
type Logger interface {
	Pure(stackoffset int, format string, v ...any)
	Info(stackoffset int, format string, v ...any)
	Warn(stackoffset int, format string, v ...any)
	Error(stackoffset int, format string, v ...any)
	Panic(stackoffset int, format string, v ...any)
	Fatal(stackoffset int, format string, v ...any)
}

// DefaultLogger default logger
type DefaultLogger struct {
	pure    *log.Logger
	info    *log.Logger
	warning *log.Logger
	err     *log.Logger
	panic   *log.Logger
	fatal   *log.Logger
}

// NewDefaultLogger create a default logger
func NewDefaultLogger(logger ...*log.Logger) *DefaultLogger {
	return &DefaultLogger{logger[0], logger[1], logger[2], logger[3], logger[4], logger[5]}
}

// Pure pure log
func (l *DefaultLogger) Pure(stackoffset int, format string, v ...any) {
	l.pure.Output(2+stackoffset, fmt.Sprintf(format, v...))
}

// Info info log
func (l *DefaultLogger) Info(stackoffset int, format string, v ...any) {
	l.info.Output(2+stackoffset, fmt.Sprintf(format, v...))
}

// Warn warning log
func (l *DefaultLogger) Warn(stackoffset int, format string, v ...any) {
	l.warning.Output(2+stackoffset, fmt.Sprintf(format, v...))
}

// Error error log
func (l *DefaultLogger) Error(stackoffset int, format string, v ...any) {
	l.err.Output(2+stackoffset, fmt.Sprintf(format, v...))
}

// Panic panic log and cause a panic
func (l *DefaultLogger) Panic(stackoffset int, format string, v ...any) {
	s := fmt.Sprintf(format, v...)
	l.panic.Output(2+stackoffset, s)
	panic(s)
}

// Fatal fatal log and exit
func (l *DefaultLogger) Fatal(stackoffset int, format string, v ...any) {
	l.fatal.Output(2+stackoffset, fmt.Sprintf(format, v...))
	os.Exit(1)
}

var logger Logger

func init() {
	flag := log.LstdFlags | log.Lmicroseconds | log.Lshortfile | log.Lmsgprefix
	logger = NewDefaultLogger(
		log.New(os.Stdout, "", 0),
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

// Pure pure log
func Pure(format string, v ...any) {
	logger.Pure(1, format, v...)
}

// Info info log
func Info(format string, v ...any) {
	logger.Info(1, format, v...)
}

// Warn warning log
func Warn(format string, v ...any) {
	logger.Warn(1, format, v...)
}

// Error error log
func Error(format string, v ...any) {
	logger.Error(1, format, v...)
}

// Panic panic log and cause a panic
func Panic(format string, v ...any) {
	logger.Panic(1, format, v...)
}

// Fatal fatal log and exit
func Fatal(format string, v ...any) {
	logger.Fatal(1, format, v...)
}
