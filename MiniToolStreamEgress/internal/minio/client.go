package minio

import (
	"context"
	"fmt"
	"io"
	"strings"

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
	client *minio.Client
	config *Config
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
		client: minioClient,
		config: config,
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

// GetObject downloads data from MinIO
func (c *Client) GetObject(ctx context.Context, subject string, objectName string) ([]byte, error) {
	bucketName := normalizeBucketName(subject)

	// Get object from MinIO
	obj, err := c.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	// Read all data
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	return data, nil
}

// GetObjectInfo returns information about an object
func (c *Client) GetObjectInfo(ctx context.Context, subject string, objectName string) (*minio.ObjectInfo, error) {
	bucketName := normalizeBucketName(subject)

	info, err := c.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	return &info, nil
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
