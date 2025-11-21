package minio

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

// Config represents MinIO repository configuration
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

// Repository represents a MinIO repository
type Repository struct {
	client        *minio.Client
	config        *Config
	logger        *logger.Logger
	bucketCache   map[string]bool
	bucketCacheMu sync.RWMutex
}

// NewRepository creates a new MinIO repository
func NewRepository(config *Config, log *logger.Logger) (*Repository, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Initialize MinIO client
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	repo := &Repository{
		client:      minioClient,
		config:      config,
		logger:      log,
		bucketCache: make(map[string]bool),
	}

	return repo, nil
}

// normalizeBucketName converts subject name to valid bucket name
// Bucket names must be lowercase and contain only letters, numbers, dots, and hyphens
func normalizeBucketName(subject string) string {
	// Replace dots with hyphens
	bucketName := strings.ReplaceAll(subject, ".", "-")
	// Convert to lowercase
	bucketName = strings.ToLower(bucketName)
	return bucketName
}

// EnsureBucket creates bucket if it doesn't exist
// For Ingress, we use a single bucket configured in settings
func (r *Repository) EnsureBucket(ctx context.Context) error {
	bucketName := r.config.BucketName

	// Check cache first
	r.bucketCacheMu.RLock()
	if r.bucketCache[bucketName] {
		r.bucketCacheMu.RUnlock()
		return nil
	}
	r.bucketCacheMu.RUnlock()

	// Check if bucket exists
	exists, err := r.client.BucketExists(ctx, bucketName)
	if err != nil {
		r.logger.Error("Failed to check bucket existence",
			logger.String("bucket", bucketName),
			logger.Error(err),
		)
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		r.logger.Info("Creating bucket",
			logger.String("bucket", bucketName),
		)
		// Create bucket
		err = r.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			r.logger.Error("Failed to create bucket",
				logger.String("bucket", bucketName),
				logger.Error(err),
			)
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		r.logger.Info("Bucket created successfully",
			logger.String("bucket", bucketName),
		)
	}

	// Update cache
	r.bucketCacheMu.Lock()
	r.bucketCache[bucketName] = true
	r.bucketCacheMu.Unlock()

	return nil
}

// UploadData uploads data to MinIO
func (r *Repository) UploadData(ctx context.Context, objectName string, data []byte, contentType string) error {
	if len(data) == 0 {
		// No data to upload
		return nil
	}

	bucketName := r.config.BucketName

	r.logger.Debug("Uploading data to MinIO",
		logger.String("bucket", bucketName),
		logger.String("object", objectName),
		logger.Int("size", len(data)),
		logger.String("content_type", contentType),
	)

	// Ensure bucket exists
	if err := r.EnsureBucket(ctx); err != nil {
		return err
	}

	// Upload object
	reader := bytes.NewReader(data)
	_, err := r.client.PutObject(ctx, bucketName, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		r.logger.Error("Failed to upload object to MinIO",
			logger.String("bucket", bucketName),
			logger.String("object", objectName),
			logger.Error(err),
		)
		return fmt.Errorf("failed to upload object: %w", err)
	}

	r.logger.Debug("Data uploaded successfully",
		logger.String("bucket", bucketName),
		logger.String("object", objectName),
	)

	return nil
}

// GetObjectURL returns the URL for accessing an object
func (r *Repository) GetObjectURL(objectName string) string {
	bucketName := r.config.BucketName
	protocol := "http"
	if r.config.UseSSL {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", protocol, r.config.Endpoint, bucketName, objectName)
}
