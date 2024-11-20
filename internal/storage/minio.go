package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"dawhub/internal/config"
	"dawhub/internal/domain"
)

const (
	presignedURLExpiry = 24 * time.Hour
	uploadTimeout      = 10 * time.Minute
	downloadTimeout    = 5 * time.Minute
)

// Common errors
var (
	ErrInvalidInput   = fmt.Errorf("invalid input provided")
	ErrUploadFailed   = fmt.Errorf("failed to upload file")
	ErrDownloadFailed = fmt.Errorf("failed to download file")
	ErrBucketNotFound = fmt.Errorf("bucket not found")
	ErrFileNotFound   = fmt.Errorf("file not found")
)

type MinioStorage struct {
	client     *minio.Client
	bucketName string
}

func NewMinioStorage(cfg config.MinioConfig) (*MinioStorage, error) {
	// Initialize MinIO client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	storage := &MinioStorage{
		client:     client,
		bucketName: cfg.Bucket,
	}

	// Ensure bucket exists
	if err := storage.ensureBucket(); err != nil {
		return nil, err
	}

	return storage, nil
}

// UploadFile uploads a file for a specific project
func (s *MinioStorage) UploadFile(projectID uint, filename string, data []byte) (string, error) {
	if projectID == 0 || filename == "" || len(data) == 0 {
		return "", ErrInvalidInput
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	// Generate object name with project path
	objectName := s.generateObjectName(projectID, filename)

	// Upload the file
	reader := bytes.NewReader(data)
	_, err := s.client.PutObject(
		ctx,
		s.bucketName,
		objectName,
		reader,
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: detectContentType(filename),
		},
	)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}

	return objectName, nil
}

// GetDownloadURL generates a presigned URL for file download
func (s *MinioStorage) GetDownloadURL(filepath string) (string, error) {
	if filepath == "" {
		return "", ErrInvalidInput
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	// Check if object exists
	_, err := s.client.StatObject(ctx, s.bucketName, filepath, minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileNotFound, err)
	}

	// Generate presigned URL
	presignedURL, err := s.client.PresignedGetObject(
		ctx,
		s.bucketName,
		filepath,
		presignedURLExpiry,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	return presignedURL.String(), nil
}

// DeleteFile removes a file from storage
func (s *MinioStorage) DeleteFile(filepath string) error {
	if filepath == "" {
		return ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	err := s.client.RemoveObject(ctx, s.bucketName, filepath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ListProjectFiles lists all files for a specific project
func (s *MinioStorage) ListProjectFiles(projectID uint) ([]string, error) {
	if projectID == 0 {
		return nil, ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	prefix := fmt.Sprintf("projects/%d/", projectID)
	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var files []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list files: %w", object.Err)
		}
		files = append(files, object.Key)
	}

	return files, nil
}

func (s *MinioStorage) GetFile(filepath string) (io.ReadCloser, domain.FileInfo, error) {
	obj, err := s.client.GetObject(
		context.Background(),
		s.bucketName,
		filepath,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, domain.FileInfo{}, fmt.Errorf("failed to get file: %w", err)
	}

	// Get stats before returning the object
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, domain.FileInfo{}, fmt.Errorf("failed to get file info: %w", err)
	}

	fileInfo := domain.FileInfo{
		Size:     stat.Size,
		Filename: path.Base(filepath),
	}

	return obj, fileInfo, nil
}

// Helper functions

// ensureBucket ensures the bucket exists, creates it if it doesn't
func (s *MinioStorage) ensureBucket() error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}

// generateObjectName creates a consistent object name structure
func (s *MinioStorage) generateObjectName(projectID uint, filename string) string {
	return fmt.Sprintf("projects/%d/%s", projectID, sanitizeFilename(filename))
}

// sanitizeFilename cleans the filename for storage
func sanitizeFilename(filename string) string {
	// Clean the path to remove any directory traversal attempts
	filename = path.Base(path.Clean(filename))
	return filename
}

// detectContentType returns the content type based on file extension
func detectContentType(filename string) string {
	ext := path.Ext(filename)
	switch ext {
	case ".mp3", ".wav", ".flac":
		return "audio/" + ext[1:]
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// Additional helper methods for monitoring and maintenance

// GetBucketSize returns the total size of all objects in the bucket
func (s *MinioStorage) GetBucketSize() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	var totalSize int64
	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return 0, fmt.Errorf("failed to calculate bucket size: %w", object.Err)
		}
		totalSize += object.Size
	}

	return totalSize, nil
}

// CleanupOldFiles removes files older than the specified duration
func (s *MinioStorage) CleanupOldFiles(age time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	cutoff := time.Now().Add(-age)
	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return fmt.Errorf("failed during cleanup: %w", object.Err)
		}

		if object.LastModified.Before(cutoff) {
			err := s.DeleteFile(object.Key)
			if err != nil {
				return fmt.Errorf("failed to delete old file %s: %w", object.Key, err)
			}
		}
	}

	return nil
}

// BackupBucket creates a backup of the entire bucket
func (s *MinioStorage) BackupBucket(targetBucket string) error {
	// Implementation would go here
	// This is just a placeholder for the interface
	return fmt.Errorf("backup functionality not implemented")
}
