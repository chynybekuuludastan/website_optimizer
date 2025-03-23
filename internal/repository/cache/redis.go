package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	// Cache key prefixes
	KeyPrefixAnalysis       = "analysis:"
	KeyPrefixWebsite        = "website:"
	KeyPrefixPopularWebsite = "popular_websites"
	KeyPrefixUserAnalyses   = "user_analyses:"
	KeyPrefixDomainStats    = "domain_stats:"

	// Default TTL for cached items
	DefaultTTL = 1 * time.Hour
)

// Repository represents a Redis cache repository
type Repository struct {
	client *redis.Client
	ctx    context.Context
}

// NewRepository creates a new Redis cache repository
func NewRepository(client *redis.Client) *Repository {
	return &Repository{
		client: client,
		ctx:    context.Background(),
	}
}

// CacheAnalysis stores an analysis in the cache
func (r *Repository) CacheAnalysis(analysis *models.Analysis) error {
	if r.client == nil {
		return nil // Skip if Redis is not available
	}

	data, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	key := KeyPrefixAnalysis + analysis.ID.String()
	return r.client.Set(r.ctx, key, data, DefaultTTL).Err()
}

// GetAnalysis retrieves an analysis from the cache
func (r *Repository) GetAnalysis(id uuid.UUID) (*models.Analysis, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	key := KeyPrefixAnalysis + id.String()
	data, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var analysis models.Analysis
	err = json.Unmarshal(data, &analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal analysis: %w", err)
	}

	return &analysis, nil
}

// CacheUserAnalyses stores a user's latest analyses in the cache
func (r *Repository) CacheUserAnalyses(userID uuid.UUID, analyses []*models.Analysis) error {
	if r.client == nil {
		return nil
	}

	data, err := json.Marshal(analyses)
	if err != nil {
		return fmt.Errorf("failed to marshal user analyses: %w", err)
	}

	key := KeyPrefixUserAnalyses + userID.String()
	return r.client.Set(r.ctx, key, data, DefaultTTL).Err()
}

// GetUserAnalyses retrieves a user's latest analyses from the cache
func (r *Repository) GetUserAnalyses(userID uuid.UUID) ([]*models.Analysis, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	key := KeyPrefixUserAnalyses + userID.String()
	data, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var analyses []*models.Analysis
	err = json.Unmarshal(data, &analyses)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user analyses: %w", err)
	}

	return analyses, nil
}

// CacheDomainStatistics stores domain statistics in the cache
func (r *Repository) CacheDomainStatistics(domain string, stats map[string]interface{}) error {
	if r.client == nil {
		return nil
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal domain statistics: %w", err)
	}

	key := KeyPrefixDomainStats + domain
	return r.client.Set(r.ctx, key, data, DefaultTTL).Err()
}

// GetDomainStatistics retrieves domain statistics from the cache
func (r *Repository) GetDomainStatistics(domain string) (map[string]interface{}, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	key := KeyPrefixDomainStats + domain
	data, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var stats map[string]interface{}
	err = json.Unmarshal(data, &stats)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal domain statistics: %w", err)
	}

	return stats, nil
}

// CachePopularWebsites stores popular websites in the cache
func (r *Repository) CachePopularWebsites(websites []*models.Website) error {
	if r.client == nil {
		return nil
	}

	data, err := json.Marshal(websites)
	if err != nil {
		return fmt.Errorf("failed to marshal popular websites: %w", err)
	}

	return r.client.Set(r.ctx, KeyPrefixPopularWebsite, data, DefaultTTL).Err()
}

// GetPopularWebsites retrieves popular websites from the cache
func (r *Repository) GetPopularWebsites() ([]*models.Website, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	data, err := r.client.Get(r.ctx, KeyPrefixPopularWebsite).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var websites []*models.Website
	err = json.Unmarshal(data, &websites)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal popular websites: %w", err)
	}

	return websites, nil
}

// InvalidateAnalysisCache removes an analysis from the cache
func (r *Repository) InvalidateAnalysisCache(id uuid.UUID) error {
	if r.client == nil {
		return nil
	}

	key := KeyPrefixAnalysis + id.String()
	return r.client.Del(r.ctx, key).Err()
}

// InvalidateUserAnalysesCache removes a user's analyses from the cache
func (r *Repository) InvalidateUserAnalysesCache(userID uuid.UUID) error {
	if r.client == nil {
		return nil
	}

	key := KeyPrefixUserAnalyses + userID.String()
	return r.client.Del(r.ctx, key).Err()
}
