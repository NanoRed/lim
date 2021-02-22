package logger

import (
	"log"
	"os"
)

// Logger application logger
// stackoffset is used to locate the position of the log in the calling stack
type Logger interface {
	Info(stackoffset int, format string, v ...interface{})
	Warn(stackoffset int, format string, v ...interface{})
	Error(stackoffset int, format string, v ...interface{})
	Panic(stackoffset int, format string, v ...interface{})
	Fatal(stackoffset int, format string, v ...interface{})
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
