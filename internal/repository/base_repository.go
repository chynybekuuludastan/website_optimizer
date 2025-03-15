package repository

import (
	"gorm.io/gorm"
)

// Repository defines common repository operations
type Repository interface {
	Create(entity interface{}) error
	FindByID(id interface{}, entity interface{}) error
	Update(entity interface{}) error
	Delete(entity interface{}) error
	Transaction(fn func(tx *gorm.DB) error) error
}

// BaseRepository implements basic repository operations
type BaseRepository struct {
	DB *gorm.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *gorm.DB) *BaseRepository {
	return &BaseRepository{DB: db}
}

// Create creates a new entity
func (r *BaseRepository) Create(entity interface{}) error {
	return r.DB.Create(entity).Error
}

// FindByID finds an entity by ID
func (r *BaseRepository) FindByID(id interface{}, entity interface{}) error {
	return r.DB.First(entity, id).Error
}

// Update updates an entity
func (r *BaseRepository) Update(entity interface{}) error {
	return r.DB.Save(entity).Error
}

// Delete deletes an entity
func (r *BaseRepository) Delete(entity interface{}) error {
	return r.DB.Delete(entity).Error
}

// Transaction runs operations in a transaction
func (r *BaseRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.DB.Transaction(fn)
}
