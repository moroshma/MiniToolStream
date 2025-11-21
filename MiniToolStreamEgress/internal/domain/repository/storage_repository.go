package repository

import "context"

// StorageRepository defines the interface for object storage operations
type StorageRepository interface {
	// GetObject downloads data from object storage
	GetObject(ctx context.Context, subject string, objectName string) ([]byte, error)

	// GetObjectURL returns the URL for accessing an object
	GetObjectURL(subject string, objectName string) string
}
