package domain

import (
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"
)

// FileMetadata contains common file attributes
type FileMetadata struct {
	Size        int64     `gorm:"not null" json:"size"`         // File size in bytes
	Filename    string    `gorm:"not null" json:"filename"`     // Original filename
	ContentType string    `gorm:"not null" json:"content_type"` // MIME type
	Hash        string    `gorm:"index" json:"hash"`            // For deduplication/integrity
	UploadedAt  time.Time `gorm:"not null" json:"uploaded_at"`  // When the file was uploaded
}

// FileInfo struct for storage service responses
type FileInfo struct {
	Size        int64
	Filename    string
	ContentType string
	Hash        string
}

// Project model with size calculations
type Project struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Main project file ID
	MainFileID *uint `json:"main_file_id"`

	// Relationships
	MainFile    *ProjectFile `gorm:"foreignKey:MainFileID" json:"main_file"`
	SampleFiles []SampleFile `gorm:"foreignKey:ProjectID" json:"sample_files"`

	// Calculated total size (updated on file changes)
	TotalSize int64 `gorm:"not null;default:0" json:"total_size"` // Total size in bytes
}

// ProjectFile model for the main project file
type ProjectFile struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	FileMetadata        // Embed common file metadata
	FilePath     string `gorm:"not null;uniqueIndex" json:"file_path"`
}

// SampleFile model with metadata
type SampleFile struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	ProjectID    uint   `json:"project_id"`
	FilePath     string `gorm:"not null;uniqueIndex" json:"file_path"`
	FileMetadata        // Embed common file metadata
}

// ProjectRepository defines the interface for project storage operations
type ProjectRepository interface {
	DB() *gorm.DB
	Create(project *Project) error
	FindAll(filters ...func(*gorm.DB) *gorm.DB) ([]Project, error)
	FindByID(id uint) (*Project, error)
	Update(project *Project) error
	Delete(id uint) error

	// File operations
	AddMainFile(projectID uint, file *ProjectFile) error
	AddSampleFile(projectID uint, file *SampleFile) error
	RemoveSampleFile(projectID uint, fileID uint) error
	GetProjectSize(projectID uint) (int64, error)

	// Batch operations
	AddSampleFiles(projectID uint, files []SampleFile) error
	RemoveSampleFiles(projectID uint, fileIDs []uint) error

	// Transaction support
	WithTx(tx *gorm.DB) ProjectRepository
	Begin() (ProjectRepository, error)
	Commit() error
	Rollback() error
}

// StorageService defines the interface for file storage operations
type StorageService interface {
	UploadFile(projectID uint, filename string, reader io.Reader) (FileInfo, string, error)
	GetDownloadURL(filepath string) (string, error)
	GetFile(filepath string) (io.ReadCloser, FileInfo, error)
	DeleteFile(filepath string) error

	// Metadata operations
	GetFileMetadata(filepath string) (FileMetadata, error)
	ValidateFile(reader io.Reader, filename string) (FileMetadata, error)

	// Batch operations
	DeleteFiles(filepaths []string) error
	ValidateFiles(files map[string]io.Reader) (map[string]FileMetadata, error)
}

// FileValidator interface for file validation operations
type FileValidator interface {
	ValidateFileType(filename string, contentType string) error
	ValidateFileSize(size int64) error
	ValidateHash(hash string) error
}

// Helper methods for Project
func (p *Project) CalculateTotalSize() int64 {
	var total int64
	if p.MainFile != nil {
		total += p.MainFile.Size
	}
	for _, sample := range p.SampleFiles {
		total += sample.Size
	}
	return total
}

// GORM Hooks
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.MainFile != nil {
		p.MainFileID = &p.MainFile.ID
	}
	return nil
}

func (p *Project) AfterCreate(tx *gorm.DB) error {
	return p.updateTotalSize(tx)
}

func (p *Project) BeforeUpdate(tx *gorm.DB) error {
	if p.MainFile != nil {
		p.MainFileID = &p.MainFile.ID
	}
	return nil
}

func (p *Project) AfterUpdate(tx *gorm.DB) error {
	return p.updateTotalSize(tx)
}

func (p *Project) updateTotalSize(tx *gorm.DB) error {
	p.TotalSize = p.CalculateTotalSize()
	return tx.Model(p).UpdateColumn("total_size", p.TotalSize).Error
}

// Project size constants
const (
	MaxProjectSize = 10 * 1024 * 1024 * 1024 // 10GB
	MaxFileSize    = 1 * 1024 * 1024 * 1024  // 1GB
	MaxSampleFiles = 100                     // Maximum number of sample files per project
)

// Error types
type ProjectError struct {
	Code    string
	Message string
}

func (e ProjectError) Error() string {
	return e.Message
}

// Common errors
var (
	ErrProjectNotFound = ProjectError{Code: "PROJECT_NOT_FOUND", Message: "project not found"}
	ErrFileNotFound    = ProjectError{Code: "FILE_NOT_FOUND", Message: "file not found"}
	ErrInvalidFileType = ProjectError{Code: "INVALID_FILE_TYPE", Message: "invalid file type"}
	ErrFileTooLarge    = ProjectError{Code: "FILE_TOO_LARGE", Message: "file too large"}
	ErrProjectTooLarge = ProjectError{Code: "PROJECT_TOO_LARGE", Message: "project size exceeds limit"}
	ErrTooManySamples  = ProjectError{Code: "TOO_MANY_SAMPLES", Message: "too many sample files"}
	ErrInvalidHash     = ProjectError{Code: "INVALID_HASH", Message: "invalid file hash"}
)

// Helper functions for file operations
func IsAllowedFileType(contentType string) bool {
	return AllowedFileTypes[contentType]
}

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Filter creators for repository queries
func PublicProjectsFilter(db *gorm.DB) *gorm.DB {
	return db.Where("is_public = ?", true)
}

func ProjectsByVersionFilter(version string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("version = ?", version)
	}
}

func ProjectsWithFilesFilter(db *gorm.DB) *gorm.DB {
	return db.Preload("MainFile").Preload("SampleFiles")
}
