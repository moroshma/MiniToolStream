package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger
type Logger struct {
	*zap.Logger
}

// Config represents logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json or console
	OutputPath string // stdout, stderr, or file path
}

// New creates a new logger instance
func New(cfg Config) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create encoder config
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	// Create output
	var output zapcore.WriteSyncer
	switch cfg.OutputPath {
	case "stdout", "":
		output = zapcore.AddSync(os.Stdout)
	case "stderr":
		output = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = zapcore.AddSync(file)
	}

	// Create core
	core := zapcore.NewCore(encoder, output, level)

	// Create logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{zapLogger}, nil
}

// WithField returns a logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.With(zap.Any(key, value))}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{l.With(zapFields...)}
}

// Helper methods for structured logging
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Uint64(key string, val uint64) zap.Field {
	return zap.Uint64(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}
