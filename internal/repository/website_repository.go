// internal/repository/website_repository.go
package repository

import (
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebsiteRepository defines operations for Website model
type WebsiteRepository interface {
	Repository
	FindByURL(url string) (*models.Website, error)
	FindAll(page, pageSize int) ([]*models.Website, int64, error)
	Search(query string, page, pageSize int) ([]*models.Website, int64, error)
	FindWithAnalyses(websiteID uuid.UUID) (*models.Website, []models.Analysis, error)
	ExistsByURL(url string) (bool, error)
}

// websiteRepository implements WebsiteRepository
type websiteRepository struct {
	*BaseRepository
}

// NewWebsiteRepository creates a new website repository
func NewWebsiteRepository(db *gorm.DB) WebsiteRepository {
	return &websiteRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// FindByURL finds a website by URL
func (r *websiteRepository) FindByURL(url string) (*models.Website, error) {
	var website models.Website
	err := r.DB.Where("url = ?", url).First(&website).Error
	if err != nil {
		return nil, err
	}
	return &website, nil
}

// FindAll retrieves all websites with pagination
func (r *websiteRepository) FindAll(page, pageSize int) ([]*models.Website, int64, error) {
	var websites []*models.Website
	var count int64

	// Count total websites
	if err := r.DB.Model(&models.Website{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get websites with pagination
	if err := r.DB.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&websites).Error; err != nil {
		return nil, 0, err
	}

	return websites, count, nil
}

// Search searches websites by URL or title
func (r *websiteRepository) Search(query string, page, pageSize int) ([]*models.Website, int64, error) {
	var websites []*models.Website
	var count int64

	// Add wildcards for LIKE query
	searchQuery := "%" + query + "%"

	// Count matching websites
	if err := r.DB.Model(&models.Website{}).
		Where("url LIKE ? OR title LIKE ?", searchQuery, searchQuery).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Search websites
	if err := r.DB.Where("url LIKE ? OR title LIKE ?", searchQuery, searchQuery).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&websites).Error; err != nil {
		return nil, 0, err
	}

	return websites, count, nil
}

// FindWithAnalyses finds a website with its analyses
func (r *websiteRepository) FindWithAnalyses(websiteID uuid.UUID) (*models.Website, []models.Analysis, error) {
	var website models.Website
	var analyses []models.Analysis

	// Get website
	if err := r.DB.First(&website, websiteID).Error; err != nil {
		return nil, nil, err
	}

	// Get analyses for this website
	if err := r.DB.Where("website_id = ?", websiteID).
		Preload("User").
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return &website, nil, err
	}

	return &website, analyses, nil
}

// ExistsByURL checks if a website with the given URL exists
func (r *websiteRepository) ExistsByURL(url string) (bool, error) {
	var count int64
	err := r.DB.Model(&models.Website{}).Where("url = ?", url).Count(&count).Error
	return count > 0, err
}
