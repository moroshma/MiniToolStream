package logger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

)

func TestNew_Success(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "json stdout debug",
			config: Config{
				Level:      "debug",
				Format:     "json",
				OutputPath: "stdout",
			},
		},
		{
			name: "console stderr info",
			config: Config{
				Level:      "info",
				Format:     "console",
				OutputPath: "stderr",
			},
		},
		{
			name: "json stdout warn",
			config: Config{
				Level:      "warn",
				Format:     "json",
				OutputPath: "",
			},
		},
		{
			name: "console stdout error",
			config: Config{
				Level:      "error",
				Format:     "console",
				OutputPath: "stdout",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := New(tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if log == nil {
				t.Fatal("expected non-nil logger")
			}
			if log.Logger == nil {
				t.Fatal("expected non-nil zap logger")
			}
		})
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	config := Config{
		Level:      "invalid-level",
		Format:     "json",
		OutputPath: "stdout",
	}

	_, err := New(config)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestNew_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := Config{
		Level:      "info",
		Format:     "json",
		OutputPath: logFile,
	}

	log, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}

	log.Info("test message")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected log file to be created")
	}
}

func TestNew_InvalidFilePath(t *testing.T) {
	config := Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "/invalid/path/that/does/not/exist/test.log",
	}

	_, err := New(config)
	if err == nil {
		t.Fatal("expected error for invalid file path")
	}
}

func TestLogger_WithField(t *testing.T) {
	config := Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "stdout",
	}

	log, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newLog := log.WithField("key", "value")
	if newLog == nil {
		t.Fatal("expected non-nil logger")
	}
	if newLog.Logger == nil {
		t.Fatal("expected non-nil zap logger")
	}
}

func TestLogger_WithFields(t *testing.T) {
	config := Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "stdout",
	}

	log, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	newLog := log.WithFields(fields)
	if newLog == nil {
		t.Fatal("expected non-nil logger")
	}
	if newLog.Logger == nil {
		t.Fatal("expected non-nil zap logger")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		field := String("key", "value")
		if field.Key != "key" {
			t.Errorf("expected key 'key', got '%s'", field.Key)
		}
	})

	t.Run("Uint64", func(t *testing.T) {
		field := Uint64("key", 123)
		if field.Key != "key" {
			t.Errorf("expected key 'key', got '%s'", field.Key)
		}
	})

	t.Run("Int", func(t *testing.T) {
		field := Int("key", 456)
		if field.Key != "key" {
			t.Errorf("expected key 'key', got '%s'", field.Key)
		}
	})

	t.Run("Bool", func(t *testing.T) {
		field := Bool("key", true)
		if field.Key != "key" {
			t.Errorf("expected key 'key', got '%s'", field.Key)
		}
	})

	t.Run("Error", func(t *testing.T) {
		err := errors.New("test error")
		field := Error(err)
		if field.Key != "error" {
			t.Errorf("expected key 'error', got '%s'", field.Key)
		}
	})
}

func TestLogger_LogLevels(t *testing.T) {
	config := Config{
		Level:      "debug",
		Format:     "json",
		OutputPath: "stdout",
	}

	log, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Debug("debug message", String("level", "debug"))
	log.Info("info message", String("level", "info"))
	log.Warn("warn message", String("level", "warn"))
	log.Error("error message", String("level", "error"))
}
