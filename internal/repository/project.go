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

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{
		db: db,
	}
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
	result := r.db.First(&project, id)

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

// Optional: Helper methods that don't need to be part of the interface
func (r *ProjectRepository) WithTx(tx *gorm.DB) *ProjectRepository {
	if tx == nil {
		return r
	}
	return &ProjectRepository{
		db: tx,
	}
}
