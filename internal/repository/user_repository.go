package repository

import (
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepository defines operations for User model
type UserRepository interface {
	Repository
	FindByEmail(email string) (*models.User, error)
	FindByUsername(username string) (*models.User, error)
	FindByRole(roleID uint) ([]*models.User, int64, error)
	FindAll(page, pageSize int) ([]*models.User, int64, error)
	FindWithActivity(userID uuid.UUID) (*models.User, []models.UserActivity, error)
	UpdatePassword(userID uuid.UUID, passwordHash string) error
	UpdateRole(userID uuid.UUID, roleID uint) error
	ExistsByEmail(email string) (bool, error)
	ExistsByUsername(username string) (bool, error)
}

// userRepository implements UserRepository
type userRepository struct {
	*BaseRepository
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB, redisClient *redis.Client) UserRepository {
	return &userRepository{
		BaseRepository: NewBaseRepository(db, redisClient),
	}
}

// FindByEmail finds a user by email
func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("email = ?", email).Preload("Role").First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername finds a user by username
func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("username = ?", username).Preload("Role").First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByRole finds users by role ID with pagination
func (r *userRepository) FindByRole(roleID uint) ([]*models.User, int64, error) {
	var users []*models.User
	var count int64

	// Count total users with this role
	if err := r.DB.Model(&models.User{}).Where("role_id = ?", roleID).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Get users with this role
	if err := r.DB.Where("role_id = ?", roleID).Preload("Role").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, count, nil
}

// FindAll retrieves all users with pagination
func (r *userRepository) FindAll(page, pageSize int) ([]*models.User, int64, error) {
	var users []*models.User
	var count int64

	// Count total users
	if err := r.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get users with pagination
	if err := r.DB.Offset(offset).Limit(pageSize).Preload("Role").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, count, nil
}

// FindWithActivity finds a user with their activity history
func (r *userRepository) FindWithActivity(userID uuid.UUID) (*models.User, []models.UserActivity, error) {
	var user models.User
	var activities []models.UserActivity

	// Get user
	if err := r.DB.Preload("Role").First(&user, userID).Error; err != nil {
		return nil, nil, err
	}

	// Get user activities
	if err := r.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&activities).Error; err != nil {
		return &user, nil, err
	}

	return &user, activities, nil
}

// UpdatePassword updates a user's password
func (r *userRepository) UpdatePassword(userID uuid.UUID, passwordHash string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error
}

// UpdateRole updates a user's role
func (r *userRepository) UpdateRole(userID uuid.UUID, roleID uint) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("role_id", roleID).Error
}

// ExistsByEmail checks if a user with the given email exists
func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.DB.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// ExistsByUsername checks if a user with the given username exists
func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.DB.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}
