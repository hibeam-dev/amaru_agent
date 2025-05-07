package util

import (
	"context"
	"errors"
	"log/slog"
	"os"
)

type Logger struct {
	slogger *slog.Logger
}

func NewLogger(level slog.Level, output *os.File) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "error" && a.Value.Kind() == slog.KindAny {
				if err, ok := a.Value.Any().(error); ok {
					var appErr *AppError
					if !errors.As(err, &appErr) {
						return slog.String(a.Key, err.Error())
					}
				}
			}
			return a
		},
	}
	handler := slog.NewJSONHandler(output, opts)

	return &Logger{
		slogger: slog.New(handler),
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slogger: l.slogger.With(args...),
	}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		slogger: l.slogger,
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.slogger.Debug(msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.slogger.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.slogger.Warn(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.slogger.Error(msg, args...)
}

func (l *Logger) LogAttrs(level slog.Level, msg string, attrs ...slog.Attr) {
	l.slogger.LogAttrs(context.Background(), level, msg, attrs...)
}

func (l *Logger) LogError(msg string, err error, args ...any) {
	if err == nil {
		return
	}

	allArgs := append([]any{"error", err}, args...)
	l.slogger.Error(msg, allArgs...)
}

var DefaultLogger = NewLogger(slog.LevelInfo, os.Stderr)

func SetDefaultLogLevel(level slog.Level) {
	DefaultLogger = NewLogger(level, os.Stderr)
}

func SetDefaultLogger(level slog.Level, output *os.File) {
	DefaultLogger = NewLogger(level, output)
}

func InitDefaultLogger() {
	DefaultLogger = NewLogger(slog.LevelInfo, os.Stderr)
}

func Debug(msg string, args ...any) {
	DefaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	DefaultLogger.Info(msg, args...)
}

func InfoAttrs(msg string, attrs ...slog.Attr) {
	DefaultLogger.LogAttrs(slog.LevelInfo, msg, attrs...)
}

func Warn(msg string, args ...any) {
	DefaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	DefaultLogger.Error(msg, args...)
}

func LogError(msg string, err error, args ...any) {
	DefaultLogger.LogError(msg, err, args...)
}

func LogErrorAttrs(msg string, err error, attrs ...slog.Attr) {
	if err == nil {
		return
	}

	errorAttr := slog.Any("error", err)
	allAttrs := append([]slog.Attr{errorAttr}, attrs...)
	DefaultLogger.LogAttrs(slog.LevelError, msg, allAttrs...)
}

func ParseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
