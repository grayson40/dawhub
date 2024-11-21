package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"dawhub/internal/domain"
	"dawhub/pkg/common"
)

type ProjectRepository struct {
	db *gorm.DB
}

var _ domain.ProjectRepository = (*ProjectRepository)(nil)

func NewProjectRepository(db *gorm.DB) domain.ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(project *domain.Project) error {
	if project == nil {
		return fmt.Errorf("%w: project is nil", common.ErrCreateFailed)
	}

	result := r.db.Create(project)
	if result.Error != nil {
		return fmt.Errorf("%w: %v", common.ErrCreateFailed, result.Error)
	}

	return nil
}

// FindAll retrieves all projects with optional filtering
func (r *ProjectRepository) FindAll(filters ...func(*gorm.DB) *gorm.DB) ([]domain.Project, error) {
	var projects []domain.Project

	query := r.db
	// Apply any filters passed in
	for _, filter := range filters {
		query = filter(query)
	}

	result := query.Find(&projects)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to fetch projects: %v", result.Error)
	}

	return projects, nil
}

func (r *ProjectRepository) FindByID(id uint) (*domain.Project, error) {
	if id == 0 {
		return nil, common.ErrInvalidID
	}

	var project domain.Project
	result := r.db.
		Preload("MainFile").
		Preload("SampleFiles").
		First(&project, id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch project: %v", result.Error)
	}

	return &project, nil
}

func (r *ProjectRepository) Update(project *domain.Project) error {
	if project == nil || project.ID == 0 {
		return fmt.Errorf("%w: invalid project", common.ErrUpdateFailed)
	}

	result := r.db.Save(project)
	if result.Error != nil {
		return fmt.Errorf("%w: %v", common.ErrUpdateFailed, result.Error)
	}

	return nil
}

func (r *ProjectRepository) Delete(id uint) error {
	if id == 0 {
		return common.ErrInvalidID
	}

	result := r.db.Delete(&domain.Project{}, id)
	if result.Error != nil {
		return fmt.Errorf("%w: %v", common.ErrDeleteFailed, result.Error)
	}

	return nil
}

// AddMainFile adds or updates the main project file
func (r *ProjectRepository) AddMainFile(projectID uint, file *domain.ProjectFile) error {
	if projectID == 0 || file == nil {
		return fmt.Errorf("%w: invalid input", common.ErrUpdateFailed)
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// First, fetch the project to ensure it exists
		var project domain.Project
		if err := tx.First(&project, projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrNotFound
			}
			return fmt.Errorf("%w: %v", common.ErrUpdateFailed, err)
		}

		// Delete existing main file if any
		if project.MainFileID != nil {
			if err := tx.Delete(&domain.ProjectFile{}, *project.MainFileID).Error; err != nil {
				return fmt.Errorf("failed to delete existing main file: %v", err)
			}
		}

		// Create new file
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("failed to create main file: %v", err)
		}

		// Update project's main file ID
		project.MainFileID = &file.ID
		if err := tx.Save(&project).Error; err != nil {
			return fmt.Errorf("failed to update project main file: %v", err)
		}

		return nil
	})
}

// AddSampleFile adds a single sample file to the project
func (r *ProjectRepository) AddSampleFile(projectID uint, file *domain.SampleFile) error {
	if projectID == 0 || file == nil {
		return fmt.Errorf("%w: invalid input", common.ErrUpdateFailed)
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check if project exists
		var project domain.Project
		if err := tx.First(&project, projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrNotFound
			}
			return fmt.Errorf("%w: %v", common.ErrUpdateFailed, err)
		}

		// Check sample files count
		var count int64
		if err := tx.Model(&domain.SampleFile{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to count sample files: %v", err)
		}

		if count >= domain.MaxSampleFiles {
			return domain.ErrTooManySamples
		}

		file.ProjectID = projectID
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("failed to create sample file: %v", err)
		}

		return nil
	})
}

// RemoveSampleFile removes a single sample file from the project
func (r *ProjectRepository) RemoveSampleFile(projectID uint, fileID uint) error {
	if projectID == 0 || fileID == 0 {
		return common.ErrInvalidID
	}

	result := r.db.Where("project_id = ? AND id = ?", projectID, fileID).Delete(&domain.SampleFile{})
	if result.Error != nil {
		return fmt.Errorf("%w: %v", common.ErrDeleteFailed, result.Error)
	}
	if result.RowsAffected == 0 {
		return common.ErrNotFound
	}

	return nil
}

// GetProjectSize calculates the total size of all project files
func (r *ProjectRepository) GetProjectSize(projectID uint) (int64, error) {
	if projectID == 0 {
		return 0, common.ErrInvalidID
	}

	var totalSize int64

	// Sum main file size
	mainFileSize := r.db.Model(&domain.Project{}).
		Select("COALESCE((SELECT size FROM project_files WHERE id = projects.main_file_id), 0)").
		Where("projects.id = ?", projectID)

	// Sum sample files size
	sampleFilesSize := r.db.Model(&domain.SampleFile{}).
		Select("COALESCE(SUM(size), 0)").
		Where("project_id = ?", projectID)

	err := r.db.Raw("SELECT (?) + (?) as total_size", mainFileSize, sampleFilesSize).
		Scan(&totalSize).Error

	if err != nil {
		return 0, fmt.Errorf("failed to calculate project size: %v", err)
	}

	return totalSize, nil
}

// AddSampleFiles adds multiple sample files in a single transaction
func (r *ProjectRepository) AddSampleFiles(projectID uint, files []domain.SampleFile) error {
	if projectID == 0 || len(files) == 0 {
		return fmt.Errorf("%w: invalid input", common.ErrUpdateFailed)
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check current sample count
		var currentCount int64
		if err := tx.Model(&domain.SampleFile{}).Where("project_id = ?", projectID).Count(&currentCount).Error; err != nil {
			return fmt.Errorf("failed to count existing samples: %v", err)
		}

		if currentCount+int64(len(files)) > domain.MaxSampleFiles {
			return domain.ErrTooManySamples
		}

		// Set project ID for all files
		for i := range files {
			files[i].ProjectID = projectID
		}

		if err := tx.Create(&files).Error; err != nil {
			return fmt.Errorf("failed to create sample files: %v", err)
		}

		return nil
	})
}

// RemoveSampleFiles removes multiple sample files in a single transaction
func (r *ProjectRepository) RemoveSampleFiles(projectID uint, fileIDs []uint) error {
	query := r.db.Where("project_id = ?", projectID)
	if fileIDs != nil {
		query = query.Where("id IN ?", fileIDs)
	}
	return query.Delete(&domain.SampleFile{}).Error
}

func (r *ProjectRepository) WithTx(tx *gorm.DB) domain.ProjectRepository {
	if tx == nil {
		return r
	}
	return &ProjectRepository{
		db: tx,
	}
}

func (r *ProjectRepository) Begin() (domain.ProjectRepository, error) {
	tx := r.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &ProjectRepository{db: tx}, nil
}

func (r *ProjectRepository) Commit() error {
	return r.db.Commit().Error
}

func (r *ProjectRepository) Rollback() error {
	return r.db.Rollback().Error
}

func (r *ProjectRepository) DB() *gorm.DB {
	return r.db
}
