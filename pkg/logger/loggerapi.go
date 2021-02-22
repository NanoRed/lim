package logger

// Logger application logger
// stackoffset is used to locate the position of the log in the calling stack
type Logger interface {
	Info(stackoffset int, format string, v ...interface{})
	Warn(stackoffset int, format string, v ...interface{})
	Error(stackoffset int, format string, v ...interface{})
	Panic(stackoffset int, format string, v ...interface{})
	Fatal(stackoffset int, format string, v ...interface{})
}
