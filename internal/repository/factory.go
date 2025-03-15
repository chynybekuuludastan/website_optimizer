package repository

import (
	"gorm.io/gorm"
)

// Factory manages all repositories
type Factory struct {
	UserRepository               UserRepository
	WebsiteRepository            WebsiteRepository
	AnalysisRepository           AnalysisRepository
	MetricsRepository            MetricsRepository
	RecommendationRepository     RecommendationRepository
	IssueRepository              IssueRepository
	ContentImprovementRepository ContentImprovementRepository
}

// NewRepositoryFactory creates a repository factory with all repositories
func NewRepositoryFactory(db *gorm.DB) *Factory {
	return &Factory{
		UserRepository:               NewUserRepository(db),
		WebsiteRepository:            NewWebsiteRepository(db),
		AnalysisRepository:           NewAnalysisRepository(db),
		MetricsRepository:            NewMetricsRepository(db),
		RecommendationRepository:     NewRecommendationRepository(db),
		IssueRepository:              NewIssueRepository(db),
		ContentImprovementRepository: NewContentImprovementRepository(db),
	}
}
