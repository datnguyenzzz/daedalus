package common

import (
	"fmt"
	"log"
	"os"
)

// Logger defines an interface for writing log messages.
type Logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

type defaultLogger struct{}

// DefaultLogger logs to the Go stdlib logs.
var DefaultLogger defaultLogger

var _ Logger = DefaultLogger

// Infof implements the Logger.Infof interface.
func (defaultLogger) Infof(format string, args ...interface{}) {
	_ = log.Output(2, fmt.Sprintf(format, args...))
}

// Errorf implements the Logger.Errorf interface.
func (defaultLogger) Errorf(format string, args ...interface{}) {
	_ = log.Output(2, fmt.Sprintf(format, args...))
}

// Fatalf implements the Logger.Fatalf interface.
func (defaultLogger) Fatalf(format string, args ...interface{}) {
	_ = log.Output(2, fmt.Sprintf(format, args...))
	os.Exit(1)
}
