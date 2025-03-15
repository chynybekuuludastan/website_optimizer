// internal/repository/metrics_repository.go
package repository

import (
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MetricsRepository defines operations for AnalysisMetric model
type MetricsRepository interface {
	Repository
	FindByAnalysisID(analysisID uuid.UUID) ([]models.AnalysisMetric, error)
	FindByCategory(analysisID uuid.UUID, category string) ([]models.AnalysisMetric, error)
	FindByName(analysisID uuid.UUID, name string) (*models.AnalysisMetric, error)
	CreateBatch(metrics []models.AnalysisMetric) error
	UpdateValue(id uuid.UUID, value datatypes.JSON) error
}

// metricsRepository implements MetricsRepository
type metricsRepository struct {
	*BaseRepository
}

// NewMetricsRepository creates a new metrics repository
func NewMetricsRepository(db *gorm.DB) MetricsRepository {
	return &metricsRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// FindByAnalysisID finds metrics by analysis ID
func (r *metricsRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.AnalysisMetric, error) {
	var metrics []models.AnalysisMetric
	err := r.DB.Where("analysis_id = ?", analysisID).Find(&metrics).Error
	return metrics, err
}

// FindByCategory finds metrics by analysis ID and category
func (r *metricsRepository) FindByCategory(analysisID uuid.UUID, category string) ([]models.AnalysisMetric, error) {
	var metrics []models.AnalysisMetric
	err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Find(&metrics).Error
	return metrics, err
}

// FindByName finds a metric by analysis ID and name
func (r *metricsRepository) FindByName(analysisID uuid.UUID, name string) (*models.AnalysisMetric, error) {
	var metric models.AnalysisMetric
	err := r.DB.Where("analysis_id = ? AND name = ?", analysisID, name).First(&metric).Error
	if err != nil {
		return nil, err
	}
	return &metric, nil
}

// CreateBatch creates multiple metrics in a batch
func (r *metricsRepository) CreateBatch(metrics []models.AnalysisMetric) error {
	return r.DB.Create(&metrics).Error
}

// UpdateValue updates a metric's value
func (r *metricsRepository) UpdateValue(id uuid.UUID, value datatypes.JSON) error {
	return r.DB.Model(&models.AnalysisMetric{}).Where("id = ?", id).Update("value", value).Error
}

// RecommendationRepository defines operations for Recommendation model
type RecommendationRepository interface {
	Repository
	FindByAnalysisID(analysisID uuid.UUID) ([]models.Recommendation, error)
	FindByCategory(analysisID uuid.UUID, category string) ([]models.Recommendation, error)
	FindByPriority(analysisID uuid.UUID, priority string) ([]models.Recommendation, error)
	CreateBatch(recommendations []models.Recommendation) error
}

// recommendationRepository implements RecommendationRepository
type recommendationRepository struct {
	*BaseRepository
}

// NewRecommendationRepository creates a new recommendation repository
func NewRecommendationRepository(db *gorm.DB) RecommendationRepository {
	return &recommendationRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// FindByAnalysisID finds recommendations by analysis ID
func (r *recommendationRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.Recommendation, error) {
	var recommendations []models.Recommendation
	err := r.DB.Where("analysis_id = ?", analysisID).Order("priority, category").Find(&recommendations).Error
	return recommendations, err
}

// FindByCategory finds recommendations by analysis ID and category
func (r *recommendationRepository) FindByCategory(analysisID uuid.UUID, category string) ([]models.Recommendation, error) {
	var recommendations []models.Recommendation
	err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Order("priority").Find(&recommendations).Error
	return recommendations, err
}

// FindByPriority finds recommendations by analysis ID and priority
func (r *recommendationRepository) FindByPriority(analysisID uuid.UUID, priority string) ([]models.Recommendation, error) {
	var recommendations []models.Recommendation
	err := r.DB.Where("analysis_id = ? AND priority = ?", analysisID, priority).Order("category").Find(&recommendations).Error
	return recommendations, err
}

// CreateBatch creates multiple recommendations in a batch
func (r *recommendationRepository) CreateBatch(recommendations []models.Recommendation) error {
	return r.DB.Create(&recommendations).Error
}

// IssueRepository defines operations for Issue model
type IssueRepository interface {
	Repository
	FindByAnalysisID(analysisID uuid.UUID) ([]models.Issue, error)
	FindByCategory(analysisID uuid.UUID, category string) ([]models.Issue, error)
	FindBySeverity(analysisID uuid.UUID, severity string) ([]models.Issue, error)
	CreateBatch(issues []models.Issue) error
}

// issueRepository implements IssueRepository
type issueRepository struct {
	*BaseRepository
}

// NewIssueRepository creates a new issue repository
func NewIssueRepository(db *gorm.DB) IssueRepository {
	return &issueRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// FindByAnalysisID finds issues by analysis ID
func (r *issueRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.DB.Where("analysis_id = ?", analysisID).Order("severity, category").Find(&issues).Error
	return issues, err
}

// FindByCategory finds issues by analysis ID and category
func (r *issueRepository) FindByCategory(analysisID uuid.UUID, category string) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Order("severity").Find(&issues).Error
	return issues, err
}

// FindBySeverity finds issues by analysis ID and severity
func (r *issueRepository) FindBySeverity(analysisID uuid.UUID, severity string) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.DB.Where("analysis_id = ? AND severity = ?", analysisID, severity).Order("category").Find(&issues).Error
	return issues, err
}

// CreateBatch creates multiple issues in a batch
func (r *issueRepository) CreateBatch(issues []models.Issue) error {
	return r.DB.Create(&issues).Error
}

// ContentImprovementRepository defines operations for ContentImprovement model
type ContentImprovementRepository interface {
	Repository
	FindByAnalysisID(analysisID uuid.UUID) ([]models.ContentImprovement, error)
	FindByElementType(analysisID uuid.UUID, elementType string) ([]models.ContentImprovement, error)
	CreateBatch(improvements []models.ContentImprovement) error
}

// contentImprovementRepository implements ContentImprovementRepository
type contentImprovementRepository struct {
	*BaseRepository
}

// NewContentImprovementRepository creates a new content improvement repository
func NewContentImprovementRepository(db *gorm.DB) ContentImprovementRepository {
	return &contentImprovementRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// FindByAnalysisID finds content improvements by analysis ID
func (r *contentImprovementRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.ContentImprovement, error) {
	var improvements []models.ContentImprovement
	err := r.DB.Where("analysis_id = ?", analysisID).Find(&improvements).Error
	return improvements, err
}

// FindByElementType finds content improvements by analysis ID and element type
func (r *contentImprovementRepository) FindByElementType(analysisID uuid.UUID, elementType string) ([]models.ContentImprovement, error) {
	var improvements []models.ContentImprovement
	err := r.DB.Where("analysis_id = ? AND element_type = ?", analysisID, elementType).Find(&improvements).Error
	return improvements, err
}

// CreateBatch creates multiple content improvements in a batch
func (r *contentImprovementRepository) CreateBatch(improvements []models.ContentImprovement) error {
	return r.DB.Create(&improvements).Error
}
