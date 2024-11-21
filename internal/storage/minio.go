package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"dawhub/internal/config"
	"dawhub/internal/domain"
	"dawhub/pkg/common"
)

const (
	presignedURLExpiry = 24 * time.Hour
	uploadTimeout      = 10 * time.Minute
	downloadTimeout    = 5 * time.Minute
)

type MinioStorage struct {
	client     *minio.Client
	bucketName string
}

func NewMinioStorage(cfg config.MinioConfig) (*MinioStorage, error) {
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

	if err := storage.ensureBucket(); err != nil {
		return nil, err
	}

	return storage, nil
}

// UploadFile handles file upload and returns file info, path, and error
func (s *MinioStorage) UploadFile(projectID uint, filename string, reader io.Reader) (domain.FileInfo, string, error) {
	log.Printf("[DEBUG] Starting file upload - ProjectID: %d, Filename: %s", projectID, filename)

	if projectID == 0 || filename == "" || reader == nil {
		log.Printf("[ERROR] Invalid input - ProjectID: %d, Filename: %s, Reader nil: %v",
			projectID, filename, reader == nil)
		return domain.FileInfo{}, "", common.ErrInvalidInput
	}

	// Read file into buffer for validation and upload
	var buffer bytes.Buffer
	tee := io.TeeReader(reader, &buffer)

	// Generate metadata and validate
	log.Printf("[DEBUG] Validating file %s", filename)
	metadata, err := s.ValidateFile(tee, filename)
	if err != nil {
		log.Printf("[ERROR] File validation failed for %s: %v", filename, err)
		return domain.FileInfo{}, "", fmt.Errorf("file validation failed: %w", err)
	}
	log.Printf("[DEBUG] File validation successful - Size: %d, ContentType: %s",
		metadata.Size, metadata.ContentType)

	// Generate object name
	objectName := s.generateObjectName(projectID, filename)
	log.Printf("[DEBUG] Generated object name: %s", objectName)

	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	log.Printf("[DEBUG] Starting MinIO upload - Bucket: %s, Object: %s, Size: %d",
		s.bucketName, objectName, metadata.Size)

	// Upload with metadata
	_, err = s.client.PutObject(
		ctx,
		s.bucketName,
		objectName,
		&buffer,
		metadata.Size,
		minio.PutObjectOptions{
			ContentType: metadata.ContentType,
			UserMetadata: map[string]string{
				"Filename":   metadata.Filename,
				"Hash":       metadata.Hash,
				"UploadedAt": metadata.UploadedAt.Format(time.RFC3339),
			},
		},
	)
	if err != nil {
		log.Printf("[ERROR] MinIO upload failed: %v", err)
		return domain.FileInfo{}, "", fmt.Errorf("upload failed: %w", err)
	}
	log.Printf("[DEBUG] MinIO upload successful")

	// Create FileInfo from metadata
	fileInfo := domain.FileInfo{
		Size:        metadata.Size,
		Filename:    filename,
		ContentType: metadata.ContentType,
		Hash:        metadata.Hash,
	}
	log.Printf("[DEBUG] Created FileInfo - Size: %d, Type: %s, Hash: %s",
		fileInfo.Size, fileInfo.ContentType, fileInfo.Hash)

	log.Printf("[INFO] File upload completed successfully - ProjectID: %d, File: %s, Path: %s",
		projectID, filename, objectName)

	return fileInfo, objectName, nil
}

func (s *MinioStorage) ValidateFile(reader io.Reader, filename string) (domain.FileMetadata, error) {
	log.Printf("[DEBUG] Starting file validation")

	var buffer bytes.Buffer
	hash := sha256.New()

	// Read file while calculating hash and size
	log.Printf("[DEBUG] Reading file and calculating hash")
	size, err := io.Copy(io.MultiWriter(&buffer, hash), reader)
	if err != nil {
		log.Printf("[ERROR] Failed to read file: %v", err)
		return domain.FileMetadata{}, fmt.Errorf("failed to process file: %w", err)
	}
	log.Printf("[DEBUG] File size: %d bytes", size)

	// Check file size
	if size > domain.MaxFileSize {
		log.Printf("[ERROR] File too large: %d bytes (max: %d)", size, domain.MaxFileSize)
		return domain.FileMetadata{}, domain.ErrFileTooLarge
	}

	// Get file hash
	fileHash := hex.EncodeToString(hash.Sum(nil))
	log.Printf("[DEBUG] File hash: %s", fileHash)

	metadata := domain.FileMetadata{
		Size:        size,
		ContentType: determineContentType(filename),
		Hash:        fileHash,
		UploadedAt:  time.Now(),
	}

	if !domain.IsAllowedFileType(metadata.ContentType) {
		log.Printf("[ERROR] Invalid content type: %s", metadata.ContentType)
		return domain.FileMetadata{}, domain.ErrInvalidFileType
	}

	log.Printf("[DEBUG] Validation successful - Size: %d, Type: %s", metadata.Size, metadata.ContentType)
	return metadata, nil
}

// GetDownloadURL generates a presigned URL for file download
func (s *MinioStorage) GetDownloadURL(filepath string) (string, error) {
	if filepath == "" {
		return "", common.ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	// Verify file exists and get metadata
	_, err := s.GetFileMetadata(filepath)
	if err != nil {
		return "", err
	}

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

// GetFile retrieves a file and its metadata
func (s *MinioStorage) GetFile(filepath string) (io.ReadCloser, domain.FileInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	// Get object stats first
	objInfo, err := s.client.StatObject(ctx, s.bucketName, filepath, minio.StatObjectOptions{})
	if err != nil {
		return nil, domain.FileInfo{}, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get the object
	obj, err := s.client.GetObject(ctx, s.bucketName, filepath, minio.GetObjectOptions{})
	if err != nil {
		return nil, domain.FileInfo{}, fmt.Errorf("failed to get file: %w", err)
	}

	fileInfo := domain.FileInfo{
		Size:        objInfo.Size,
		Filename:    objInfo.UserMetadata["Filename"],
		ContentType: objInfo.ContentType,
		Hash:        objInfo.UserMetadata["Hash"],
	}

	return obj, fileInfo, nil
}

// GetFileMetadata retrieves file metadata without downloading the file
func (s *MinioStorage) GetFileMetadata(filepath string) (domain.FileMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	objInfo, err := s.client.StatObject(ctx, s.bucketName, filepath, minio.StatObjectOptions{})
	if err != nil {
		return domain.FileMetadata{}, fmt.Errorf("failed to get metadata: %w", err)
	}

	uploadedAt, _ := time.Parse(time.RFC3339, objInfo.UserMetadata["UploadedAt"])

	return domain.FileMetadata{
		Size:        objInfo.Size,
		Filename:    objInfo.UserMetadata["Filename"],
		ContentType: objInfo.ContentType,
		Hash:        objInfo.UserMetadata["Hash"],
		UploadedAt:  uploadedAt,
	}, nil
}

// DeleteFile removes a single file
func (s *MinioStorage) DeleteFile(filepath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	err := s.client.RemoveObject(ctx, s.bucketName, filepath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// DeleteFiles removes multiple files in parallel
func (s *MinioStorage) DeleteFiles(filepaths []string) error {
	if len(filepaths) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for _, filepath := range filepaths {
			objectsCh <- minio.ObjectInfo{
				Key: filepath,
			}
		}
	}()

	for err := range s.client.RemoveObjects(ctx, s.bucketName, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return fmt.Errorf("failed to delete files: %w", err.Err)
		}
	}

	return nil
}

// ValidateFiles validates multiple files in parallel
func (s *MinioStorage) ValidateFiles(files map[string]io.Reader) (map[string]domain.FileMetadata, error) {
	if len(files) == 0 {
		return nil, nil
	}

	results := make(map[string]domain.FileMetadata)
	errors := make([]error, 0)
	var mutex sync.Mutex
	var wg sync.WaitGroup

	for filename, reader := range files {
		wg.Add(1)
		go func(fname string, r io.Reader) {
			defer wg.Done()

			metadata, err := s.ValidateFile(r, filename)
			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				errors = append(errors, fmt.Errorf("failed to validate %s: %w", fname, err))
				return
			}

			metadata.Filename = fname
			results[fname] = metadata
		}(filename, reader)
	}

	wg.Wait()

	if len(errors) > 0 {
		return nil, fmt.Errorf("validation errors: %v", errors)
	}

	return results, nil
}

// Helper functions remain the same
func (s *MinioStorage) generateObjectName(projectID uint, filename string) string {
	return fmt.Sprintf("projects/%d/%s", projectID, sanitizeFilename(filename))
}

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

func sanitizeFilename(filename string) string {
	return path.Base(path.Clean(filename))
}

func determineContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".flp":
		return "audio/x-flp"
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".aiff":
		return "audio/aiff"
	case ".aif":
		return "audio/aiff"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
