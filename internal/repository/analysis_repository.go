// internal/repository/analysis_repository.go
package repository

import (
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AnalysisRepository defines operations for Analysis model
type AnalysisRepository interface {
	Repository
	FindByUserID(userID uuid.UUID, page, pageSize int) ([]*models.Analysis, int64, error)
	FindByWebsiteID(websiteID uuid.UUID, page, pageSize int) ([]*models.Analysis, int64, error)
	FindPublic(page, pageSize int) ([]*models.Analysis, int64, error)
	FindByStatus(status string, page, pageSize int) ([]*models.Analysis, int64, error)
	FindWithDetails(analysisID uuid.UUID) (*models.Analysis, error)
	UpdateStatus(analysisID uuid.UUID, status string) error
	SetPublic(analysisID uuid.UUID, isPublic bool) error
	FindWithMetricsByCategory(analysisID uuid.UUID, category string) (*models.Analysis, []models.AnalysisMetric, error)
	FindWithRecommendations(analysisID uuid.UUID) (*models.Analysis, []models.Recommendation, error)
	FindWithIssues(analysisID uuid.UUID) (*models.Analysis, []models.Issue, error)
	CountByDateRange(startDate, endDate time.Time) (int64, error)
	FindByDateRange(startDate, endDate time.Time, page, pageSize int) ([]*models.Analysis, int64, error)
	FindLatestByUserID(userID uuid.UUID, limit int) ([]*models.Analysis, error)
	UpdateMetadata(analysisID uuid.UUID, metadata datatypes.JSON) error
	CountByStatusAndDate(status string, startDate, endDate time.Time) (int64, error)
}

// analysisRepository implements AnalysisRepository
type analysisRepository struct {
	*BaseRepository
}

// NewAnalysisRepository creates a new analysis repository
func NewAnalysisRepository(db *gorm.DB, redisClient *redis.Client) AnalysisRepository {
	return &analysisRepository{
		BaseRepository: NewBaseRepository(db, redisClient),
	}
}

// FindByUserID finds analyses by user ID with pagination
func (r *analysisRepository) FindByUserID(userID uuid.UUID, page, pageSize int) ([]*models.Analysis, int64, error) {
	var analyses []*models.Analysis
	var count int64

	// Count analyses for this user
	if err := r.DB.Model(&models.Analysis{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get analyses with pagination
	if err := r.DB.Where("user_id = ?", userID).
		Preload("Website").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return nil, 0, err
	}

	return analyses, count, nil
}

// FindByWebsiteID finds analyses by website ID with pagination
func (r *analysisRepository) FindByWebsiteID(websiteID uuid.UUID, page, pageSize int) ([]*models.Analysis, int64, error) {
	var analyses []*models.Analysis
	var count int64

	// Count analyses for this website
	if err := r.DB.Model(&models.Analysis{}).Where("website_id = ?", websiteID).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get analyses with pagination
	if err := r.DB.Where("website_id = ?", websiteID).
		Preload("User").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return nil, 0, err
	}

	return analyses, count, nil
}

// FindPublic finds public analyses with pagination
func (r *analysisRepository) FindPublic(page, pageSize int) ([]*models.Analysis, int64, error) {
	var analyses []*models.Analysis
	var count int64

	// Count public analyses
	if err := r.DB.Model(&models.Analysis{}).Where("is_public = ?", true).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get public analyses with pagination
	if err := r.DB.Where("is_public = ?", true).
		Preload("Website").
		Preload("User").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return nil, 0, err
	}

	return analyses, count, nil
}

// FindByStatus finds analyses by status with pagination
func (r *analysisRepository) FindByStatus(status string, page, pageSize int) ([]*models.Analysis, int64, error) {
	var analyses []*models.Analysis
	var count int64

	// Count analyses with this status
	if err := r.DB.Model(&models.Analysis{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get analyses with pagination
	if err := r.DB.Where("status = ?", status).
		Preload("Website").
		Preload("User").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return nil, 0, err
	}

	return analyses, count, nil
}

// FindWithDetails finds an analysis with all related data
func (r *analysisRepository) FindWithDetails(analysisID uuid.UUID) (*models.Analysis, error) {
	var analysis models.Analysis

	err := r.DB.
		Preload("Website").
		Preload("User").
		Preload("User.Role").
		Preload("Metrics").
		Preload("Recommendations").
		Preload("ContentImprovements").
		Preload("Issues").
		First(&analysis, analysisID).Error

	if err != nil {
		return nil, err
	}

	return &analysis, nil
}

// UpdateStatus updates an analysis status
func (r *analysisRepository) UpdateStatus(analysisID uuid.UUID, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// If status is completed, set completed_at
	if status == "completed" {
		updates["completed_at"] = time.Now()
	}

	return r.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(updates).Error
}

// SetPublic sets the public flag for an analysis
func (r *analysisRepository) SetPublic(analysisID uuid.UUID, isPublic bool) error {
	return r.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Update("is_public", isPublic).Error
}

// FindWithMetricsByCategory finds an analysis with metrics for a specific category
func (r *analysisRepository) FindWithMetricsByCategory(analysisID uuid.UUID, category string) (*models.Analysis, []models.AnalysisMetric, error) {
	var analysis models.Analysis
	var metrics []models.AnalysisMetric

	// Get analysis
	if err := r.DB.Preload("Website").Preload("User").First(&analysis, analysisID).Error; err != nil {
		return nil, nil, err
	}

	// Get metrics for this category
	if err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Find(&metrics).Error; err != nil {
		return &analysis, nil, err
	}

	return &analysis, metrics, nil
}

// FindWithRecommendations finds an analysis with its recommendations
func (r *analysisRepository) FindWithRecommendations(analysisID uuid.UUID) (*models.Analysis, []models.Recommendation, error) {
	var analysis models.Analysis
	var recommendations []models.Recommendation

	// Get analysis
	if err := r.DB.Preload("Website").Preload("User").First(&analysis, analysisID).Error; err != nil {
		return nil, nil, err
	}

	// Get recommendations
	if err := r.DB.Where("analysis_id = ?", analysisID).Order("priority, category").Find(&recommendations).Error; err != nil {
		return &analysis, nil, err
	}

	return &analysis, recommendations, nil
}

// FindWithIssues finds an analysis with its issues
func (r *analysisRepository) FindWithIssues(analysisID uuid.UUID) (*models.Analysis, []models.Issue, error) {
	var analysis models.Analysis
	var issues []models.Issue

	// Get analysis
	if err := r.DB.Preload("Website").Preload("User").First(&analysis, analysisID).Error; err != nil {
		return nil, nil, err
	}

	// Get issues
	if err := r.DB.Where("analysis_id = ?", analysisID).Order("severity, category").Find(&issues).Error; err != nil {
		return &analysis, nil, err
	}

	return &analysis, issues, nil
}

// CountByDateRange counts analyses created within a date range
func (r *analysisRepository) CountByDateRange(startDate, endDate time.Time) (int64, error) {
	var count int64
	err := r.DB.Model(&models.Analysis{}).
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Count(&count).Error
	return count, err
}

// FindByDateRange finds analyses created within a date range with pagination
func (r *analysisRepository) FindByDateRange(startDate, endDate time.Time, page, pageSize int) ([]*models.Analysis, int64, error) {
	var analyses []*models.Analysis
	var count int64

	// Count analyses within date range
	if err := r.DB.Model(&models.Analysis{}).
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get analyses within date range with pagination
	if err := r.DB.Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Preload("Website").
		Preload("User").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&analyses).Error; err != nil {
		return nil, 0, err
	}

	return analyses, count, nil
}

// FindLatestByUserID finds the most recent analyses for a specific user with caching
func (r *analysisRepository) FindLatestByUserID(userID uuid.UUID, limit int) ([]*models.Analysis, error) {
	// Try to get from cache if available
	if r.CacheRepo != nil {
		cachedAnalyses, err := r.CacheRepo.GetUserAnalyses(userID)
		if err == nil && cachedAnalyses != nil && len(cachedAnalyses) >= limit {
			return cachedAnalyses[:limit], nil
		}
	}

	// Otherwise, fetch from database
	var analyses []*models.Analysis

	err := r.DB.Where("user_id = ?", userID).
		Preload("Website").
		Order("created_at DESC").
		Limit(limit).
		Find(&analyses).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find latest analyses: %w", err)
	}

	// Cache the result for future requests
	if r.CacheRepo != nil && len(analyses) > 0 {
		go r.CacheRepo.CacheUserAnalyses(userID, analyses)
	}

	return analyses, nil
}

// UpdateMetadata updates the metadata field of an analysis
func (r *analysisRepository) UpdateMetadata(analysisID uuid.UUID, metadata datatypes.JSON) error {
	result := r.DB.Model(&models.Analysis{}).
		Where("id = ?", analysisID).
		Update("metadata", metadata)

	if result.Error != nil {
		return fmt.Errorf("failed to update analysis metadata: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("analysis not found: %s", analysisID)
	}

	return nil
}

// CountByStatusAndDate counts analyses by status within a date range
func (r *analysisRepository) CountByStatusAndDate(status string, startDate, endDate time.Time) (int64, error) {
	var count int64

	query := r.DB.Model(&models.Analysis{})

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count analyses by status and date: %w", err)
	}

	return count, nil
}
