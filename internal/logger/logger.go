package logger

import (
	"fmt"
	"io"
	"os"
)

// Logger defines the interface for logging throughout the application
type Logger interface {
	// Info logs informational messages
	Info(format string, args ...interface{})

	// Success logs success messages (typically with ✅)
	Success(format string, args ...interface{})

	// Warn logs warning messages (typically with ⚠️)
	Warn(format string, args ...interface{})

	// Error logs error messages (typically with ❌)
	Error(format string, args ...interface{})

	// Progress logs progress/action messages (typically with 📦 or 🗑️)
	Progress(format string, args ...interface{})

	// Print logs plain messages without any prefix
	Print(format string, args ...interface{})

	// Println logs plain messages with a newline
	Println(args ...interface{})
}

// StdLogger is the default logger implementation that writes to stdout
type StdLogger struct {
	out io.Writer
}

// NewStdLogger creates a new StdLogger that writes to the provided writer
func NewStdLogger(out io.Writer) *StdLogger {
	return &StdLogger{out: out}
}

// Default returns a StdLogger that writes to os.Stdout
func Default() *StdLogger {
	return NewStdLogger(os.Stdout)
}

// Info logs informational messages
func (l *StdLogger) Info(format string, args ...interface{}) {
	fmt.Fprintf(l.out, format, args...)
}

// Success logs success messages with ✅ prefix
func (l *StdLogger) Success(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "✅ "+format, args...)
}

// Warn logs warning messages with ⚠️ prefix
func (l *StdLogger) Warn(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "⚠️  "+format, args...)
}

// Error logs error messages with ❌ prefix
func (l *StdLogger) Error(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "❌ "+format, args...)
}

// Progress logs progress/action messages with 📦 prefix
func (l *StdLogger) Progress(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "📦 "+format, args...)
}

// Print logs plain messages without any prefix
func (l *StdLogger) Print(format string, args ...interface{}) {
	fmt.Fprintf(l.out, format, args...)
}

// Println logs plain messages with a newline
func (l *StdLogger) Println(args ...interface{}) {
	fmt.Fprintln(l.out, args...)
}

// NopLogger is a logger that discards all output (useful for testing)
type NopLogger struct{}

// NewNopLogger creates a new NopLogger
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

func (l *NopLogger) Info(format string, args ...interface{})     {}
func (l *NopLogger) Success(format string, args ...interface{})  {}
func (l *NopLogger) Warn(format string, args ...interface{})     {}
func (l *NopLogger) Error(format string, args ...interface{})    {}
func (l *NopLogger) Progress(format string, args ...interface{}) {}
func (l *NopLogger) Print(format string, args ...interface{})    {}
func (l *NopLogger) Println(args ...interface{})                 {}
