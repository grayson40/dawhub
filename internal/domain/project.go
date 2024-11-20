package domain

import (
	"io"
	"time"

	"gorm.io/gorm"
)

type FileInfo struct {
	Size     int64
	Filename string
}

type Project struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `json:"name"`
	FilePath  string    `json:"file_path"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectRepository defines the interface for project storage operations
type ProjectRepository interface {
	Create(project *Project) error
	FindAll(filters ...func(*gorm.DB) *gorm.DB) ([]Project, error) // Updated to include filters
	FindByID(id uint) (*Project, error)
	Update(project *Project) error
	Delete(id uint) error
}

// StorageService defines the interface for file storage operations
type StorageService interface {
	UploadFile(projectID uint, filename string, data []byte) (string, error)
	GetDownloadURL(filepath string) (string, error)
	GetFile(filepath string) (io.ReadCloser, FileInfo, error)
	DeleteFile(filepath string) error
}
