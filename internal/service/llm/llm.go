package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

// Logger interface for service logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// Common errors
var (
	ErrAPIRequestFailed   = errors.New("LLM API request failed")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrResponseProcessing = errors.New("failed to process LLM response")
	ErrInvalidProvider    = errors.New("invalid LLM provider specified")
	ErrCacheMiss          = errors.New("cache miss")
	ErrTimeout            = errors.New("request timed out")
	ErrCancelled          = errors.New("request was cancelled")
)

// DefaultLogger provides a basic implementation of the Logger interface
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, keysAndValues)
}

func (l *DefaultLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Printf("[INFO] %s %v", msg, keysAndValues)
}

func (l *DefaultLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, keysAndValues)
}

// ProgressCallback is a function that receives generation progress updates
type ProgressCallback func(progress float64, message string)

// Service handles LLM API interactions with caching and rate limiting
type Service struct {
	providers       map[string]Provider
	defaultProvider string
	redisClient     *redis.Client
	limiter         *rate.Limiter
	cacheTTL        time.Duration
	maxRetries      int
	retryDelay      time.Duration
	mutex           sync.RWMutex
	logger          Logger
	defaultTimeout  time.Duration
}

// ServiceOptions contains configuration for the LLM service
type ServiceOptions struct {
	DefaultProvider string
	RedisClient     *redis.Client
	RateLimit       rate.Limit
	RateBurst       int
	CacheTTL        time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	Logger          Logger
	DefaultTimeout  time.Duration
}

// NewService creates a new LLM service with the specified options
func NewService(opts ServiceOptions) *Service {
	// Set default values if not provided
	if opts.CacheTTL == 0 {
		opts.CacheTTL = 24 * time.Hour
	}
	if opts.RateLimit == 0 {
		opts.RateLimit = rate.Limit(10) // 10 requests per second by default
	}
	if opts.RateBurst == 0 {
		opts.RateBurst = 1
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 1 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = &DefaultLogger{}
	}
	if opts.DefaultTimeout == 0 {
		opts.DefaultTimeout = 30 * time.Second
	}

	return &Service{
		providers:       make(map[string]Provider),
		defaultProvider: opts.DefaultProvider,
		redisClient:     opts.RedisClient,
		limiter:         rate.NewLimiter(opts.RateLimit, opts.RateBurst),
		cacheTTL:        opts.CacheTTL,
		maxRetries:      opts.MaxRetries,
		retryDelay:      opts.RetryDelay,
		logger:          opts.Logger,
		defaultTimeout:  opts.DefaultTimeout,
	}
}

// Provider interface for LLM providers
type Provider interface {
	// GenerateContent generates improved content based on the request
	GenerateContent(ctx context.Context, request *ContentRequest) (*ContentResponse, error)

	// GenerateContentWithProgress generates content with progress updates
	GenerateContentWithProgress(ctx context.Context, request *ContentRequest, progressCb ProgressCallback) (*ContentResponse, error)

	// GenerateHTML generates HTML code for the improved content
	GenerateHTML(ctx context.Context, original string, improved *ContentResponse) (string, error)

	// GetName returns the name of the provider
	GetName() string

	// Close performs any necessary cleanup
	Close() error
}

// RegisterProvider registers an LLM provider with the service
func (s *Service) RegisterProvider(provider Provider) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	providerName := provider.GetName()
	s.providers[providerName] = provider

	if s.defaultProvider == "" {
		s.defaultProvider = providerName
	}

	s.logger.Info("Registered LLM provider", "provider", providerName)
}

// GetProvider returns a provider by name, using the default if name is empty
func (s *Service) GetProvider(name string) (Provider, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if name == "" {
		name = s.defaultProvider
	}

	provider, exists := s.providers[name]
	if !exists {
		return nil, ErrInvalidProvider
	}

	return provider, nil
}

// generateCacheKey creates a cache key from the request
func (s *Service) generateCacheKey(request *ContentRequest, operation string) string {
	// Add language to the cache key if specified
	langPart := ""
	if request.Language != "" && request.Language != "en" {
		langPart = ":" + request.Language
	}

	// Add target audience to the cache key if specified
	audiencePart := ""
	if request.TargetAudience != "" {
		audiencePart = ":" + request.TargetAudience
	}

	return fmt.Sprintf("llm:%s:%s%s%s", operation, request.URL, langPart, audiencePart)
}

// getFromCache retrieves a response from Redis cache
func (s *Service) getFromCache(ctx context.Context, key string) (*ContentResponse, error) {
	if s.redisClient == nil {
		return nil, ErrCacheMiss
	}

	data, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, ErrCacheMiss
	}

	var response ContentResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		s.logger.Error("Failed to unmarshal cached response", "error", err, "key", key)
		return nil, ErrCacheMiss
	}

	return &response, nil
}

// saveToCache saves a response to Redis cache
func (s *Service) saveToCache(ctx context.Context, key string, response *ContentResponse) error {
	if s.redisClient == nil {
		return nil
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.redisClient.Set(ctx, key, data, s.cacheTTL).Err()
}

// GenerateContent generates improved content with caching, rate limiting and retries
func (s *Service) GenerateContent(ctx context.Context, request *ContentRequest, providerName string) (*ContentResponse, error) {
	startTime := time.Now()

	// Apply timeout if not already set
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, s.defaultTimeout)
		defer cancel()
	}

	// Try to get from cache first
	cacheKey := s.generateCacheKey(request, "content")
	if s.redisClient != nil {
		cachedResponse, err := s.getFromCache(ctx, cacheKey)
		if err == nil {
			// Cache hit
			cachedResponse.CachedResult = true
			cachedResponse.ProcessingTime = time.Since(startTime)

			s.logger.Debug("Cache hit for content generation",
				"url", request.URL,
				"provider", cachedResponse.ProviderUsed)

			return cachedResponse, nil
		}
	}

	// Apply rate limiting
	if err := s.limiter.Wait(ctx); err != nil {
		s.logger.Error("Rate limit exceeded", "error", err)
		return nil, ErrRateLimitExceeded
	}

	// Get provider
	provider, err := s.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	// Execute with retries
	var response *ContentResponse
	var lastErr error

	for retry := 0; retry <= s.maxRetries; retry++ {
		if retry > 0 {
			// Log retry attempt
			s.logger.Info("Retrying LLM API request",
				"attempt", retry,
				"provider", provider.GetName(),
				"url", request.URL)

			// Wait before retry with exponential backoff
			select {
			case <-time.After(s.retryDelay * time.Duration(1<<uint(retry-1))):
				// Continue after delay
			case <-ctx.Done():
				switch ctx.Err() {
				case context.Canceled:
					return nil, ErrCancelled
				case context.DeadlineExceeded:
					return nil, ErrTimeout
				default:
					return nil, ctx.Err()
				}
			}
		}

		// Generate content
		response, lastErr = provider.GenerateContent(ctx, request)
		if lastErr == nil {
			break
		}

		// Check if we should continue retrying
		if errors.Is(lastErr, context.Canceled) {
			return nil, ErrCancelled
		} else if errors.Is(lastErr, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}

		// Log error
		s.logger.Error("LLM API request failed",
			"error", lastErr,
			"provider", provider.GetName(),
			"retry", retry)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIRequestFailed, lastErr)
	}

	// Set metadata
	response.ProviderUsed = provider.GetName()
	response.ProcessingTime = time.Since(startTime)
	response.CachedResult = false

	// Cache the result
	if s.redisClient != nil {
		if err := s.saveToCache(ctx, cacheKey, response); err != nil {
			s.logger.Error("Failed to cache LLM response", "error", err)
		}
	}

	// Log success
	s.logger.Info("Generated content successfully",
		"provider", provider.GetName(),
		"url", request.URL,
		"time", response.ProcessingTime)

	return response, nil
}

// GenerateContentWithProgress generates content with progress updates
func (s *Service) GenerateContentWithProgress(ctx context.Context, request *ContentRequest,
	providerName string, progressCb ProgressCallback) (*ContentResponse, error) {
	startTime := time.Now()

	// Apply timeout if not already set
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, s.defaultTimeout)
		defer cancel()
	}

	// Try to get from cache first
	cacheKey := s.generateCacheKey(request, "content")
	if s.redisClient != nil {
		cachedResponse, err := s.getFromCache(ctx, cacheKey)
		if err == nil {
			// Cache hit
			cachedResponse.CachedResult = true
			cachedResponse.ProcessingTime = time.Since(startTime)

			// Report 100% progress for cached responses
			if progressCb != nil {
				progressCb(100, "Retrieved from cache")
			}

			s.logger.Debug("Cache hit for content generation with progress",
				"url", request.URL,
				"provider", cachedResponse.ProviderUsed)

			return cachedResponse, nil
		}

		// Report cache miss
		if progressCb != nil {
			progressCb(0, "Cache miss, generating new content")
		}
	}

	// Apply rate limiting
	if err := s.limiter.Wait(ctx); err != nil {
		s.logger.Error("Rate limit exceeded", "error", err)
		return nil, ErrRateLimitExceeded
	}

	// Get provider
	provider, err := s.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	// Generate content with progress
	response, err := provider.GenerateContentWithProgress(ctx, request, progressCb)
	if err != nil {
		// Check for specific errors
		if errors.Is(err, context.Canceled) {
			return nil, ErrCancelled
		} else if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}

		return nil, fmt.Errorf("%w: %v", ErrAPIRequestFailed, err)
	}

	// Set metadata
	response.ProviderUsed = provider.GetName()
	response.ProcessingTime = time.Since(startTime)
	response.CachedResult = false

	// Cache the result
	if s.redisClient != nil {
		if err := s.saveToCache(ctx, cacheKey, response); err != nil {
			s.logger.Error("Failed to cache LLM response", "error", err)
		}
	}

	// Report 100% progress
	if progressCb != nil {
		progressCb(100, "Content generation completed")
	}

	// Log success
	s.logger.Info("Generated content with progress successfully",
		"provider", provider.GetName(),
		"url", request.URL,
		"time", response.ProcessingTime)

	return response, nil
}

// GenerateHTML generates HTML for improved content with caching and retries
func (s *Service) GenerateHTML(ctx context.Context, request *ContentRequest,
	improved *ContentResponse, providerName string) (string, error) {

	// Apply timeout if not already set
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, s.defaultTimeout)
		defer cancel()
	}

	// Try to get from cache first
	cacheKey := s.generateCacheKey(request, "html")
	if s.redisClient != nil {
		cachedHTML, err := s.redisClient.Get(ctx, cacheKey).Result()
		if err == nil && cachedHTML != "" {
			s.logger.Debug("Cache hit for HTML generation", "url", request.URL)
			return cachedHTML, nil
		}
	}

	// Apply rate limiting
	if err := s.limiter.Wait(ctx); err != nil {
		s.logger.Error("Rate limit exceeded for HTML generation", "error", err)
		return "", ErrRateLimitExceeded
	}

	// Get provider
	provider, err := s.GetProvider(providerName)
	if err != nil {
		return "", err
	}

	// Generate HTML with retries
	var html string
	var lastErr error

	for retry := 0; retry <= s.maxRetries; retry++ {
		if retry > 0 {
			// Log retry attempt
			s.logger.Info("Retrying HTML generation",
				"attempt", retry,
				"provider", provider.GetName())

			// Wait before retry with exponential backoff
			select {
			case <-time.After(s.retryDelay * time.Duration(1<<uint(retry-1))):
				// Continue after delay
			case <-ctx.Done():
				switch ctx.Err() {
				case context.Canceled:
					return "", ErrCancelled
				case context.DeadlineExceeded:
					return "", ErrTimeout
				default:
					return "", ctx.Err()
				}
			}
		}

		// Generate HTML
		html, lastErr = provider.GenerateHTML(ctx, request.Content, improved)
		if lastErr == nil {
			break
		}

		// Check if we should continue retrying
		if errors.Is(lastErr, context.Canceled) {
			return "", ErrCancelled
		} else if errors.Is(lastErr, context.DeadlineExceeded) {
			return "", ErrTimeout
		}

		// Log error
		s.logger.Error("HTML generation failed",
			"error", lastErr,
			"provider", provider.GetName(),
			"retry", retry)
	}

	if lastErr != nil {
		return "", fmt.Errorf("%w: %v", ErrAPIRequestFailed, lastErr)
	}

	// Cache the result
	if s.redisClient != nil && html != "" {
		if err := s.redisClient.Set(ctx, cacheKey, html, s.cacheTTL).Err(); err != nil {
			s.logger.Error("Failed to cache HTML", "error", err)
		}
	}

	// Update the response with the HTML
	improved.HTML = html

	// Log success
	s.logger.Info("Generated HTML successfully", "provider", provider.GetName())

	return html, nil
}

// ExtractContentFromText tries to extract content from non-JSON text
func ExtractContentFromText(text string) (map[string]string, error) {
	result := make(map[string]string)

	// Try to find heading
	headingRegex := regexp.MustCompile(`(?i)заголовок|heading[:\s]+["']?([^"'\n]+)["']?`)
	if matches := headingRegex.FindStringSubmatch(text); len(matches) > 1 {
		result["heading"] = strings.TrimSpace(matches[1])
	}

	// Try to find CTA
	ctaRegex := regexp.MustCompile(`(?i)cta[:\s]+["']?([^"'\n]+)["']?`)
	if matches := ctaRegex.FindStringSubmatch(text); len(matches) > 1 {
		result["cta_button"] = strings.TrimSpace(matches[1])
	}

	// Try to find content
	contentRegex := regexp.MustCompile(`(?i)контент|content[:\s]+["']?([^"'\n]+)["']?`)
	if matches := contentRegex.FindStringSubmatch(text); len(matches) > 1 {
		result["improved_content"] = strings.TrimSpace(matches[1])
	}

	// If we haven't found anything, just use the whole text as content
	if len(result) == 0 {
		result["improved_content"] = strings.TrimSpace(text)
	}

	return result, nil
}

// CleanCodeBlocks removes markdown code blocks from text
func CleanCodeBlocks(text string) string {
	// Remove markdown code block markers
	codeBlocksRegex := regexp.MustCompile("(?s)```(html)?(.+?)```")
	if matches := codeBlocksRegex.FindStringSubmatch(text); len(matches) > 2 {
		return strings.TrimSpace(matches[2])
	}

	// If no code blocks found, return the original text
	return strings.TrimSpace(text)
}

// GetAvailableProviders returns a list of available provider names
func (s *Service) GetAvailableProviders() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	providers := make([]string, 0, len(s.providers))
	for name := range s.providers {
		providers = append(providers, name)
	}

	return providers
}

// Close closes all providers
func (s *Service) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var errs []string
	for name, provider := range s.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %s", strings.Join(errs, "; "))
	}

	return nil
}
