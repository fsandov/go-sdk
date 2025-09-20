package logs

import (
	"context"
	"os"
	"sync"

	"github.com/fsandov/go-sdk/pkg/env"
	"github.com/fsandov/go-sdk/pkg/notifiers"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *Logger
	initOnce     sync.Once
)

type Logger struct {
	zap       *zap.Logger
	notifiers map[string][]notifiers.Notifier
	appName   string
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

func NewLogger(opts ...zap.Option) *Logger {
	initOnce.Do(func() {
		opts = append(opts, zap.AddCallerSkip(3))

		var zapLogger *zap.Logger
		if env.IsRemote() {
			cfg := zap.NewProductionConfig()
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			zapLogger, _ = cfg.Build(append(opts, zap.AddCaller())...)
		} else {
			cfg := zap.NewDevelopmentConfig()
			zapLogger, _ = cfg.Build(append(opts, zap.AddCaller())...)
		}

		globalLogger = &Logger{
			zap:       zapLogger,
			notifiers: make(map[string][]notifiers.Notifier),
			appName:   os.Getenv("APP_NAME"),
		}
		zap.ReplaceGlobals(zapLogger)
	})
	return globalLogger
}

func GetLogger() *Logger {
	if globalLogger == nil {
		return NewLogger()
	}
	return globalLogger
}

func (l *Logger) AddNotifier(level string, notifier notifiers.Notifier) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.notifiers[level] = append(l.notifiers[level], notifier)
}

func Info(ctx context.Context, msg string, fieldsAndOpts ...any) {
	GetLogger().Info(ctx, msg, fieldsAndOpts...)
}
func Warn(ctx context.Context, msg string, fieldsAndOpts ...any) {
	GetLogger().Warn(ctx, msg, fieldsAndOpts...)
}
func Error(ctx context.Context, msg string, fieldsAndOpts ...any) {
	GetLogger().Error(ctx, msg, fieldsAndOpts...)
}
func Debug(ctx context.Context, msg string, fieldsAndOpts ...any) {
	GetLogger().Debug(ctx, msg, fieldsAndOpts...)
}

func (l *Logger) Info(ctx context.Context, msg string, fieldsAndOpts ...any) {
	l.logWithOpts(ctx, "info", msg, fieldsAndOpts...)
}
func (l *Logger) Warn(ctx context.Context, msg string, fieldsAndOpts ...any) {
	l.logWithOpts(ctx, "warn", msg, fieldsAndOpts...)
}
func (l *Logger) Error(ctx context.Context, msg string, fieldsAndOpts ...any) {
	l.logWithOpts(ctx, "error", msg, fieldsAndOpts...)
}
func (l *Logger) Debug(ctx context.Context, msg string, fieldsAndOpts ...any) {
	l.logWithOpts(ctx, "debug", msg, fieldsAndOpts...)
}

func (l *Logger) logWithOpts(ctx context.Context, level, msg string, fieldsAndOpts ...any) {
	var zapFields []zap.Field
	opts := &logOptions{}
	for _, item := range fieldsAndOpts {
		switch v := item.(type) {
		case []zap.Field:
			zapFields = append(zapFields, v...)
		case zap.Field:
			zapFields = append(zapFields, v)
		case LogOption:
			v.apply(opts)
		default:
			l.zap.Debug("unsupported log field type", zap.Any("field", v))
		}
	}

	if l.appName != "" {
		msg = "[" + l.appName + "] " + msg
	}

	switch level {
	case "info":
		l.zap.Info(msg, zapFields...)
	case "warn":
		l.zap.Warn(msg, zapFields...)
	case "error":
		l.zap.Error(msg, zapFields...)
	case "debug":
		l.zap.Debug(msg, zapFields...)
	}
	if opts.withNotifier {
		l.sendNotifications(ctx, level, msg, zapFields)
	}
}

func (l *Logger) sendNotifications(ctx context.Context, level, msg string, fields []zap.Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	notifiersForLevel := l.notifiers[level]
	if len(notifiersForLevel) == 0 {
		l.zap.Warn("No notifiers configured for level", zap.String("level", level))
		return
	}
	fieldMap := fieldsToMap(fields)
	for _, notifier := range notifiersForLevel {
		l.wg.Add(1)
		go func(n notifiers.Notifier) {
			defer l.wg.Done()
			if err := n.Notify(ctx, level, msg, fieldMap); err != nil {
				l.zap.Error("failed to send notification", zap.String("level", level), zap.Error(err))
			}
		}(notifier)
	}
}

func fieldsToMap(fields []zap.Field) map[string]any {
	out := map[string]any{}
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	for k, v := range enc.Fields {
		out[k] = v
	}
	return out
}

func (l *Logger) Flush() {
	l.wg.Wait()
}
func Flush() {
	globalLogger.Flush()
}
