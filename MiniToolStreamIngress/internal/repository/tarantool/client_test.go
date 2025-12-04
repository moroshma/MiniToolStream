package tarantool

import (
	"testing"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

func TestNewRepository_NilConfig(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	_, err := NewRepository(nil, log)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if err.Error() != "config cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestToUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected uint64
	}{
		{
			name:     "uint64",
			input:    uint64(123),
			expected: 123,
		},
		{
			name:     "int64",
			input:    int64(456),
			expected: 456,
		},
		{
			name:     "int",
			input:    int(789),
			expected: 789,
		},
		{
			name:     "int8",
			input:    int8(12),
			expected: 12,
		},
		{
			name:     "int16",
			input:    int16(345),
			expected: 345,
		},
		{
			name:     "int32",
			input:    int32(6789),
			expected: 6789,
		},
		{
			name:     "uint",
			input:    uint(111),
			expected: 111,
		},
		{
			name:     "uint8",
			input:    uint8(222),
			expected: 222,
		},
		{
			name:     "uint16",
			input:    uint16(333),
			expected: 333,
		},
		{
			name:     "uint32",
			input:    uint32(444),
			expected: 444,
		},
		{
			name:     "float64",
			input:    float64(555.7),
			expected: 555,
		},
		{
			name:     "float32",
			input:    float32(666.8),
			expected: 666,
		},
		{
			name:     "string (default case)",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "nil (default case)",
			input:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toUint64(tt.input)
			if result != tt.expected {
				t.Errorf("toUint64(%v) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRepository_PublishMessage_EmptySubject(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})
	config := &Config{
		Address: "localhost:3301",
		Timeout: 5 * time.Second,
	}

	repo := &Repository{
		config: config,
		logger: log,
		closed: false,
	}

	_, err := repo.PublishMessage("", nil)
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
	if err.Error() != "subject cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_PublishMessage_NilHeaders(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})
	config := &Config{
		Address: "localhost:3301",
		Timeout: 5 * time.Second,
	}

	repo := &Repository{
		config: config,
		logger: log,
		closed: true,
	}

	_, err := repo.PublishMessage("test.subject", nil)
	if err == nil {
		t.Fatal("expected error for closed repository")
	}
}

func TestRepository_Ping_Closed(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})
	config := &Config{
		Address: "localhost:3301",
		Timeout: 5 * time.Second,
	}

	repo := &Repository{
		config: config,
		logger: log,
		closed: true,
	}

	err := repo.Ping()
	if err == nil {
		t.Fatal("expected error for closed repository")
	}
	if err.Error() != "repository is closed" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Close_AlreadyClosed(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})
	config := &Config{
		Address: "localhost:3301",
		Timeout: 5 * time.Second,
	}

	repo := &Repository{
		config: config,
		logger: log,
		closed: true,
	}

	err := repo.Close()
	if err != nil {
		t.Errorf("expected no error when closing already closed repository, got: %v", err)
	}
}
