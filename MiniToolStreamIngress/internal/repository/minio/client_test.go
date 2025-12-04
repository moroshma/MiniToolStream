package minio

import (
	"context"
	"testing"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

func TestNormalizeBucketName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "test",
			expected: "test",
		},
		{
			name:     "uppercase to lowercase",
			input:    "TEST",
			expected: "test",
		},
		{
			name:     "dots to hyphens",
			input:    "test.bucket.name",
			expected: "test-bucket-name",
		},
		{
			name:     "mixed case and dots",
			input:    "Test.Bucket.Name",
			expected: "test-bucket-name",
		},
		{
			name:     "already normalized",
			input:    "test-bucket-name",
			expected: "test-bucket-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBucketName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeBucketName(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

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

func TestRepository_GetObjectURL(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		objectName string
		expected   string
	}{
		{
			name: "http without SSL",
			config: &Config{
				Endpoint:   "localhost:9000",
				UseSSL:     false,
				BucketName: "test-bucket",
			},
			objectName: "test-object",
			expected:   "http://localhost:9000/test-bucket/test-object",
		},
		{
			name: "https with SSL",
			config: &Config{
				Endpoint:   "minio.example.com",
				UseSSL:     true,
				BucketName: "production-bucket",
			},
			objectName: "important-file.txt",
			expected:   "https://minio.example.com/production-bucket/important-file.txt",
		},
		{
			name: "object with path",
			config: &Config{
				Endpoint:   "localhost:9000",
				UseSSL:     false,
				BucketName: "mybucket",
			},
			objectName: "folder/subfolder/file.pdf",
			expected:   "http://localhost:9000/mybucket/folder/subfolder/file.pdf",
		},
	}

	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{
				config: tt.config,
				logger: log,
			}

			url := repo.GetObjectURL(tt.objectName)
			if url != tt.expected {
				t.Errorf("GetObjectURL() = %s, want %s", url, tt.expected)
			}
		})
	}
}

func TestRepository_UploadData_EmptyData(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})
	config := &Config{
		Endpoint:   "localhost:9000",
		BucketName: "test-bucket",
	}

	repo := &Repository{
		config:      config,
		logger:      log,
		bucketCache: make(map[string]bool),
	}

	ctx := context.Background()
	err := repo.UploadData(ctx, "test-object", []byte{}, "text/plain")
	if err != nil {
		t.Errorf("expected no error for empty data, got: %v", err)
	}
}
