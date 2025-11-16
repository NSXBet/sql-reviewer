package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// Interface defines logging methods used by the advisor
type Interface interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
}

// Logger implements the logging interface
type Logger struct {
	logger *slog.Logger
}

// New creates a new logger instance with colored output
func New() *Logger {
	handler := tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.Kitchen,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Color error attributes (both key and value) in red (color code 9)
			if a.Key == "error" {
				return tint.Attr(9, a)
			}
			// Check if the value is an error type
			if a.Value.Kind() == slog.KindAny {
				if _, ok := a.Value.Any().(error); ok {
					return tint.Attr(9, a)
				}
			}
			return a
		},
	})
	return &Logger{
		logger: slog.New(handler),
	}
}

// NewWithLevel creates a new logger with specified level and colored output
func NewWithLevel(level slog.Level) *Logger {
	handler := tint.NewHandler(os.Stderr, &tint.Options{
		Level:      level,
		TimeFormat: time.Kitchen,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Color error attributes (both key and value) in red (color code 9)
			if a.Key == "error" {
				return tint.Attr(9, a)
			}
			// Check if the value is an error type
			if a.Value.Kind() == slog.KindAny {
				if _, ok := a.Value.Any().(error); ok {
					return tint.Attr(9, a)
				}
			}
			return a
		},
	})
	return &Logger{
		logger: slog.New(handler),
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// GetSlogLogger returns the underlying slog logger
func (l *Logger) GetSlogLogger() *slog.Logger {
	return l.logger
}

// Error creates a structured error field
func Error(err error) slog.Attr {
	return slog.Any("error", err)
}

// Stack creates a structured stack field
func Stack(stack string) slog.Attr {
	return slog.String("stack", stack)
}
