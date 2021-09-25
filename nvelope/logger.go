package nvelope

import (
	"fmt"
)

// BasicLogger is just the start of what a logger might
// support.  It exists mostly as a placeholder.  Future
// versions of nvelope will prefer more capabile loggers
// but will use type assertions so that the BasicLogger
// will remain acceptable to the APIs.
type BasicLogger interface {
	Debug(msg string, fields ...map[string]interface{})
	Error(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
}

// StdLogger is implmented by the base library log.Logger
type StdLogger interface {
	Print(v ...interface{})
}

type wrappedStdLogger struct {
	log StdLogger
}

// LoggerFromStd creates a
func LoggerFromStd(log StdLogger) func() BasicLogger {
	return func() BasicLogger {
		return wrappedStdLogger{log: log}
	}
}

func (std wrappedStdLogger) Error(msg string, fields ...map[string]interface{}) {
	if len(fields) == 0 {
		std.log.Print(msg)
		return
	}
	vals := make([]interface{}, 1, len(fields)*4+1)
	vals[0] = msg
	for _, m := range fields {
		for k, v := range m {
			vals = append(vals, k+"="+fmt.Sprint(v))
		}
	}
	std.log.Print(vals...)
}

func (std wrappedStdLogger) Warn(msg string, fields ...map[string]interface{}) {
	std.Error(msg, fields...)
}
func (std wrappedStdLogger) Debug(msg string, fields ...map[string]interface{}) {
	std.Error(msg, fields...)
}

// NoLogger injects a BasicLogger that discards all inputs
func NoLogger() BasicLogger {
	return nilLogger{}
}

type nilLogger struct{}

var _ BasicLogger = nilLogger{}

func (_ nilLogger) Error(msg string, fields ...map[string]interface{}) {}
func (_ nilLogger) Warn(msg string, fields ...map[string]interface{})  {}
func (_ nilLogger) Debug(msg string, fields ...map[string]interface{}) {}
