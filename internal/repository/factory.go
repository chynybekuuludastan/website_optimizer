package repository

import (
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"github.com/chynybekuuludastan/website_optimizer/internal/repository/cache"
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
	CacheRepository              *cache.Repository
}

// NewRepositoryFactory creates a repository factory with all repositories
func NewRepositoryFactory(db *gorm.DB, redisClient *redis.Client) *Factory {
	return &Factory{
		UserRepository:               NewUserRepository(db, redisClient),
		WebsiteRepository:            NewWebsiteRepository(db, redisClient),
		AnalysisRepository:           NewAnalysisRepository(db, redisClient),
		MetricsRepository:            NewMetricsRepository(db, redisClient),
		RecommendationRepository:     NewRecommendationRepository(db, redisClient),
		IssueRepository:              NewIssueRepository(db, redisClient),
		ContentImprovementRepository: NewContentImprovementRepository(db, redisClient),
		CacheRepository:              cache.NewRepository(redisClient),
	}
}
