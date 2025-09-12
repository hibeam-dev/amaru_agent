package util

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
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

func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		slogger: l.slogger.With("component", component),
	}
}

func (l *Logger) Debug(msg string, fields map[string]any) {
	if fields == nil {
		l.slogger.Debug(msg)
		return
	}
	l.slogger.Debug(msg, mapToAttrs(fields)...)
}

func (l *Logger) Info(msg string, fields map[string]any) {
	if fields == nil {
		l.slogger.Info(msg)
		return
	}
	l.slogger.Info(msg, mapToAttrs(fields)...)
}

func (l *Logger) Warn(msg string, fields map[string]any) {
	if fields == nil {
		l.slogger.Warn(msg)
		return
	}
	l.slogger.Warn(msg, mapToAttrs(fields)...)
}

func (l *Logger) Error(msg string, fields map[string]any) {
	if fields == nil {
		l.slogger.Error(msg)
		return
	}
	l.slogger.Error(msg, mapToAttrs(fields)...)
}

func (l *Logger) LogAttrs(level slog.Level, msg string, attrs ...slog.Attr) {
	l.slogger.LogAttrs(context.Background(), level, msg, attrs...)
}

func (l *Logger) LogError(msg string, err error, fields map[string]any) {
	if err == nil {
		return
	}

	if fields == nil {
		fields = make(map[string]any)
	}
	fields["error"] = err
	l.Error(msg, fields)
}

func mapToAttrs(fields map[string]any) []any {
	if len(fields) == 0 {
		return nil
	}

	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return attrs
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

func Debug(msg string, fields map[string]any) {
	DefaultLogger.Debug(msg, fields)
}

func With(args ...any) *Logger {
	return DefaultLogger.With(args...)
}

func Info(msg string, fields map[string]any) {
	DefaultLogger.Info(msg, fields)
}

func Warn(msg string, fields map[string]any) {
	DefaultLogger.Warn(msg, fields)
}

func Error(msg string, fields map[string]any) {
	DefaultLogger.Error(msg, fields)
}

func LogError(msg string, err error, fields map[string]any) {
	DefaultLogger.LogError(msg, err, fields)
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

type WireGuardLogWriter struct {
	logger *Logger
	prefix string
}

func NewWireGuardLogWriter(logger *Logger, prefix string) *WireGuardLogWriter {
	return &WireGuardLogWriter{
		logger: logger,
		prefix: prefix,
	}
}

func (w *WireGuardLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	msg = strings.TrimPrefix(msg, w.prefix)
	msg = strings.TrimSpace(msg)

	if strings.Contains(msg, "ERROR") || strings.Contains(msg, "error") {
		w.logger.Error("wireguard", map[string]any{"message": msg})
	} else if strings.Contains(msg, "WARN") || strings.Contains(msg, "warn") {
		w.logger.Warn("wireguard", map[string]any{"message": msg})
	} else {
		w.logger.Debug("wireguard", map[string]any{"message": msg})
	}

	return len(p), nil
}

var _ io.Writer = (*WireGuardLogWriter)(nil)
