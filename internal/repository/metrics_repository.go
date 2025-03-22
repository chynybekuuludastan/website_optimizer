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

func (r *recommendationRepository) FindByCategory(analysisID uuid.UUID, category string) ([]models.Recommendation, error) {
	var recommendations []models.Recommendation

	err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Order("priority").
		Find(&recommendations).Error

	// Установка правильных ID для связанных сущностей
	for i := range recommendations {
		if recommendations[i].Analysis.ID == uuid.Nil {
			recommendations[i].Analysis.ID = analysisID
		}
	}

	return recommendations, err
}

func (r *recommendationRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.Recommendation, error) {
	var recommendations []models.Recommendation

	err := r.DB.Where("analysis_id = ?", analysisID).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Order("priority, category").
		Find(&recommendations).Error

	// Установка правильных ID для связанных сущностей
	for i := range recommendations {
		if recommendations[i].Analysis.ID == uuid.Nil {
			recommendations[i].Analysis.ID = analysisID
		}
	}

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

func (r *issueRepository) FindByCategory(analysisID uuid.UUID, category string) ([]models.Issue, error) {
	var issues []models.Issue

	err := r.DB.Where("analysis_id = ? AND category = ?", analysisID, category).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Preload("Analysis.User.Role").
		Order("severity").
		Find(&issues).Error

	// При получении данных для модели Issue, необходимо убедиться,
	// что связанные ID правильно заполнены
	for i := range issues {
		// Убедимся, что анализу присвоен правильный ID
		if issues[i].Analysis.ID == uuid.Nil {
			issues[i].Analysis.ID = analysisID
		}
	}

	return issues, err
}

func (r *issueRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.Issue, error) {
	var issues []models.Issue

	err := r.DB.Where("analysis_id = ?", analysisID).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Preload("Analysis.User.Role").
		Order("severity, category").
		Find(&issues).Error

	for i := range issues {
		if issues[i].Analysis.ID == uuid.Nil {
			issues[i].Analysis.ID = analysisID
		}
	}

	return issues, err
}
