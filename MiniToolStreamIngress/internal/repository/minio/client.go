package minio

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config represents MinIO client configuration
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

// Client represents a MinIO client
type Client struct {
	client        *minio.Client
	config        *Config
	bucketCache   map[string]bool
	bucketCacheMu sync.RWMutex
}

// NewClient creates a new MinIO client
func NewClient(config *Config) (*Client, error) {
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

	client := &Client{
		client:      minioClient,
		config:      config,
		bucketCache: make(map[string]bool),
	}

	return client, nil
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
func (c *Client) EnsureBucket(ctx context.Context, subject string) (string, error) {
	bucketName := normalizeBucketName(subject)

	// Check cache first
	c.bucketCacheMu.RLock()
	if c.bucketCache[bucketName] {
		c.bucketCacheMu.RUnlock()
		return bucketName, nil
	}
	c.bucketCacheMu.RUnlock()

	// Check if bucket exists
	exists, err := c.client.BucketExists(ctx, bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		// Create bucket
		err = c.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Update cache
	c.bucketCacheMu.Lock()
	c.bucketCache[bucketName] = true
	c.bucketCacheMu.Unlock()

	return bucketName, nil
}

// UploadData uploads data to MinIO
func (c *Client) UploadData(ctx context.Context, subject string, objectName string, data []byte, contentType string) error {
	if len(data) == 0 {
		// No data to upload
		return nil
	}

	// Ensure bucket exists
	bucketName, err := c.EnsureBucket(ctx, subject)
	if err != nil {
		return err
	}

	// Upload object
	reader := bytes.NewReader(data)
	_, err = c.client.PutObject(ctx, bucketName, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// GetObjectURL returns the URL for accessing an object
func (c *Client) GetObjectURL(subject string, objectName string) string {
	bucketName := normalizeBucketName(subject)
	protocol := "http"
	if c.config.UseSSL {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", protocol, c.config.Endpoint, bucketName, objectName)
}
