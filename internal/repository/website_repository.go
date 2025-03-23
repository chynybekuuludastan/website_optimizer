package repository

import (
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/go-redis/redis/v8"
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
	FindDomainStatistics(domain string) (map[string]interface{}, error)
	FindPopularWebsites(limit int) ([]*models.Website, error)
}

// websiteRepository implements WebsiteRepository
type websiteRepository struct {
	*BaseRepository
}

// NewWebsiteRepository creates a new website repository
func NewWebsiteRepository(db *gorm.DB, redisClient *redis.Client) WebsiteRepository {
	return &websiteRepository{
		BaseRepository: NewBaseRepository(db, redisClient),
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

// FindDomainStatistics gathers statistics for a specific domain
func (r *websiteRepository) FindDomainStatistics(domain string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total analyses for this domain
	var analysesCount int64
	err := r.DB.Model(&models.Analysis{}).
		Joins("JOIN websites ON analyses.website_id = websites.id").
		Where("websites.url LIKE ?", "%"+domain+"%").
		Count(&analysesCount).Error

	if err != nil {
		return nil, fmt.Errorf("failed to count domain analyses: %w", err)
	}

	stats["analyses_count"] = analysesCount

	// Get average score if available in metadata
	var avgScore float64
	err = r.DB.Model(&models.Analysis{}).
		Joins("JOIN websites ON analyses.website_id = websites.id").
		Where("websites.url LIKE ?", "%"+domain+"%").
		Select("AVG(CAST(metadata->>'overall_score' AS FLOAT))").
		Row().Scan(&avgScore)

	// Ignore error as the score might not be available in all analyses
	if err == nil {
		stats["average_score"] = avgScore
	}

	// Get last analysis date
	var lastAnalysis time.Time
	err = r.DB.Model(&models.Analysis{}).
		Joins("JOIN websites ON analyses.website_id = websites.id").
		Where("websites.url LIKE ?", "%"+domain+"%").
		Order("analyses.created_at DESC").
		Limit(1).
		Select("analyses.created_at").
		Row().Scan(&lastAnalysis)

	if err == nil && !lastAnalysis.IsZero() {
		stats["last_analysis_date"] = lastAnalysis
	}

	return stats, nil
}

// FindPopularWebsites finds the most frequently analyzed websites with caching
func (r *websiteRepository) FindPopularWebsites(limit int) ([]*models.Website, error) {
	// Try to get from cache if available
	if r.CacheRepo != nil {
		cachedWebsites, err := r.CacheRepo.GetPopularWebsites()
		if err == nil && cachedWebsites != nil && len(cachedWebsites) >= limit {
			return cachedWebsites[:limit], nil
		}
	}

	// Otherwise, fetch from database
	var websites []*models.Website

	err := r.DB.Model(&models.Website{}).
		Select("websites.*, COUNT(analyses.id) as analysis_count").
		Joins("LEFT JOIN analyses ON websites.id = analyses.website_id").
		Group("websites.id").
		Order("analysis_count DESC").
		Limit(limit).
		Find(&websites).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find popular websites: %w", err)
	}

	// Cache the result for future requests
	if r.CacheRepo != nil && len(websites) > 0 {
		go r.CacheRepo.CachePopularWebsites(websites)
	}

	return websites, nil
}
