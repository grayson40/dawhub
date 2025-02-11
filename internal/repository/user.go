package repository

import (
	"dawhub/internal/domain"
	"errors"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) GetByID(id uint) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(username string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(user *domain.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&domain.User{}, id).Error
}

func (r *UserRepository) List() ([]domain.User, error) {
	var users []domain.User
	if err := r.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) CreateBetaUser(betaUser *domain.BetaUser) error {
	return r.db.Create(betaUser).Error
}

func (r *UserRepository) GetBetaUserByEmail(email string) (*domain.BetaUser, error) {
	var betaUser domain.BetaUser
	if err := r.db.First(&betaUser, email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &betaUser, nil
}

func (r *UserRepository) GetAllBetaUsers(page, limit int) ([]domain.BetaUser, error) {
	var betaUsers []domain.BetaUser
	if err := r.db.Offset((page - 1) * limit).Limit(limit).Find(&betaUsers).Error; err != nil {
		return nil, err
	}
	return betaUsers, nil
}

func (r *UserRepository) CountBetaUsers(count *int64) error {
	return r.db.Model(&domain.BetaUser{}).Count(count).Error
}
