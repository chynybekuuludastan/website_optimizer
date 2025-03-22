package repository

import (
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ContentImprovementRepository interface {
	Repository
	FindByAnalysisID(analysisID uuid.UUID) ([]models.ContentImprovement, error)
	FindByElementType(analysisID uuid.UUID, elementType string) ([]models.ContentImprovement, error)
	CreateBatch(improvements []models.ContentImprovement) error
}

type contentImprovementRepository struct {
	*BaseRepository
}

func NewContentImprovementRepository(db *gorm.DB) ContentImprovementRepository {
	return &contentImprovementRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

func (r *contentImprovementRepository) FindByAnalysisID(analysisID uuid.UUID) ([]models.ContentImprovement, error) {
	var improvements []models.ContentImprovement

	err := r.DB.Where("analysis_id = ?", analysisID).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Find(&improvements).Error

	// Установка правильных ID для связанных сущностей
	for i := range improvements {
		if improvements[i].Analysis.ID == uuid.Nil {
			improvements[i].Analysis.ID = analysisID
		}
	}

	return improvements, err
}

func (r *contentImprovementRepository) FindByElementType(analysisID uuid.UUID, elementType string) ([]models.ContentImprovement, error) {
	var improvements []models.ContentImprovement

	err := r.DB.Where("analysis_id = ? AND element_type = ?", analysisID, elementType).
		Preload("Analysis", func(db *gorm.DB) *gorm.DB {
			return db.Omit("Metrics", "Recommendations", "ContentImprovements", "Issues")
		}).
		Preload("Analysis.Website").
		Preload("Analysis.User").
		Find(&improvements).Error

	// Установка правильных ID для связанных сущностей
	for i := range improvements {
		if improvements[i].Analysis.ID == uuid.Nil {
			improvements[i].Analysis.ID = analysisID
		}
	}

	return improvements, err
}

func (r *contentImprovementRepository) CreateBatch(improvements []models.ContentImprovement) error {
	return r.DB.Create(&improvements).Error
}
