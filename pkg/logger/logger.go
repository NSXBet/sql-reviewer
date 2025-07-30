package logger

import (
	"log/slog"
	"os"
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

// New creates a new logger instance
func New() *Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		logger: slog.New(handler),
	}
}

// NewWithLevel creates a new logger with specified level
func NewWithLevel(level slog.Level) *Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
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
