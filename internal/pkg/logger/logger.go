package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var (
	globalLogger *slog.Logger
	once         sync.Once
)

func Init(level string) {
	once.Do(func() {
		var logLevel slog.Level
		switch level {
		case "debug":
			logLevel = slog.LevelDebug
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelInfo
		}

		// Use JSON handler for production-ready structured logging
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
		globalLogger = slog.New(handler)
		slog.SetDefault(globalLogger)
	})
}

// Get returns the global logger instance
func Get() *slog.Logger {
	if globalLogger == nil {
		Init("info")
	}
	return globalLogger
}

// Helper functions for quick logging
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

func With(args ...any) *slog.Logger {
	return Get().With(args...)
}

func LogError(ctx context.Context, err error, msg string, args ...any) {
	if err == nil {
		return
	}
	// Add error to attributes
	args = append(args, slog.String("error", err.Error()))
	Get().ErrorContext(ctx, msg, args...)
}
