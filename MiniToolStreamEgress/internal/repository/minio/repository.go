package minio

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	pkglogger "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
)

// Repository implements domain.StorageRepository using MinIO
type Repository struct {
	client *minio.Client
	config *Config
	logger *pkglogger.Logger
}

// Config represents MinIO repository configuration
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

// NewRepository creates a new MinIO repository
func NewRepository(cfg *Config, log *pkglogger.Logger) (*Repository, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Initialize MinIO client
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	repo := &Repository{
		client: minioClient,
		config: cfg,
		logger: log,
	}

	return repo, nil
}

// normalizeBucketName converts subject name to valid bucket name
func normalizeBucketName(subject string) string {
	bucketName := strings.ReplaceAll(subject, ".", "-")
	bucketName = strings.ToLower(bucketName)
	return bucketName
}

// GetObject downloads data from MinIO
func (r *Repository) GetObject(ctx context.Context, subject string, objectName string) ([]byte, error) {
	bucketName := r.config.BucketName

	r.logger.Debug("Getting object from MinIO",
		pkglogger.String("bucket", bucketName),
		pkglogger.String("object", objectName),
	)

	// Get object from MinIO
	obj, err := r.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	// Read all data
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	r.logger.Debug("Object retrieved successfully",
		pkglogger.String("bucket", bucketName),
		pkglogger.String("object", objectName),
		pkglogger.Int("size", len(data)),
	)

	return data, nil
}

// GetObjectURL returns the URL for accessing an object
func (r *Repository) GetObjectURL(subject string, objectName string) string {
	bucketName := r.config.BucketName
	protocol := "http"
	if r.config.UseSSL {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", protocol, r.config.Endpoint, bucketName, objectName)
}
