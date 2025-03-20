package lighthouse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

// Constants for API configuration
const (
	DefaultTimeout     = 60 * time.Second
	DefaultRetries     = 3
	DefaultCacheTTL    = 24 * time.Hour
	DefaultRateLimit   = 5 // Requests per second
	DefaultMaxParallel = 3
)

// Category represents a Lighthouse audit category
type Category string

const (
	CategoryPerformance   Category = "performance"
	CategoryAccessibility Category = "accessibility"
	CategoryBestPractices Category = "best-practices"
	CategorySEO           Category = "seo"
	CategoryPWA           Category = "pwa" // Progressive Web App
)

// FormFactor represents the device form factor for testing
type FormFactor string

const (
	FormFactorMobile  FormFactor = "mobile"
	FormFactorDesktop FormFactor = "desktop"
)

// AuditOptions represents options for Lighthouse audits
type AuditOptions struct {
	Categories     []Category
	FormFactor     FormFactor
	Locale         string
	CacheTTL       time.Duration
	OnlyCategories bool      // Only return category scores, not individual audits
	Strategy       string    // "desktop" or "mobile" for PSI API
	URLSharding    bool      // Whether to perform URL sharding for batch processing
	Throttling     bool      // Whether to apply CPU/network throttling
	MaxWaitTime    int       // Maximum time to wait for results in seconds
	EmulatedDevice string    // Specific device to emulate
	ReferenceTime  time.Time // Time to use for reference in caching
	SkipAudits     []string  // Audits to skip
	OnlyAudits     []string  // Only run these audits
}

// DefaultAuditOptions returns default audit options
func DefaultAuditOptions() AuditOptions {
	return AuditOptions{
		Categories: []Category{
			CategoryPerformance,
			CategoryAccessibility,
			CategoryBestPractices,
			CategorySEO,
		},
		FormFactor:     FormFactorMobile,
		Locale:         "en",
		CacheTTL:       DefaultCacheTTL,
		OnlyCategories: false,
		Strategy:       "mobile",
		URLSharding:    false,
		Throttling:     true,
		MaxWaitTime:    60,
	}
}

// MetricsResult represents the performance metrics from Lighthouse
type MetricsResult struct {
	FirstContentfulPaint    float64 `json:"first_contentful_paint"`
	LargestContentfulPaint  float64 `json:"largest_contentful_paint"`
	FirstMeaningfulPaint    float64 `json:"first_meaningful_paint"`
	SpeedIndex              float64 `json:"speed_index"`
	TimeToInteractive       float64 `json:"time_to_interactive"`
	TotalBlockingTime       float64 `json:"total_blocking_time"`
	CumulativeLayoutShift   float64 `json:"cumulative_layout_shift"`
	FirstCPUIdle            float64 `json:"first_cpu_idle"`
	MaxPotentialFID         float64 `json:"max_potential_fid"`
	ServerResponseTime      float64 `json:"server_response_time"`
	RenderBlockingResources int     `json:"render_blocking_resources"`
	DOMSize                 int     `json:"dom_size"`
	NetworkRequests         int     `json:"network_requests"`
	TotalByteWeight         int     `json:"total_byte_weight"`
}

// Audit represents a single Lighthouse audit
type Audit struct {
	ID               string                 `json:"id"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	Score            float64                `json:"score"`
	ScoreDisplayMode string                 `json:"scoreDisplayMode"`
	DisplayValue     string                 `json:"displayValue,omitempty"`
	NumericValue     float64                `json:"numericValue,omitempty"`
	Details          map[string]interface{} `json:"details,omitempty"`
	Warnings         []string               `json:"warnings,omitempty"`
}

// AuditResult represents the full result of a Lighthouse audit
type AuditResult struct {
	LighthouseVersion string                   `json:"lighthouseVersion"`
	FetchTime         string                   `json:"fetchTime"`
	URL               string                   `json:"requestedUrl"`
	FinalURL          string                   `json:"finalUrl"`
	TotalAnalysisTime int                      `json:"analysisUTCTimestamp"`
	Scores            map[string]float64       `json:"scores"`
	Metrics           MetricsResult            `json:"metrics"`
	Audits            map[string]Audit         `json:"audits"`
	Categories        map[string]interface{}   `json:"categories"`
	Issues            []map[string]interface{} `json:"issues"`
	Recommendations   []string                 `json:"recommendations"`
}

// Client represents a Lighthouse API client
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	redisClient *redis.Client
	limiter     *rate.Limiter
	retries     int
	cacheTTL    time.Duration
	maxParallel int
	mu          sync.Mutex
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client for the Lighthouse client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithRedisClient sets the Redis client for caching
func WithRedisClient(redisClient *redis.Client) ClientOption {
	return func(c *Client) {
		c.redisClient = redisClient
	}
}

// WithRetries sets the number of retries for failed requests
func WithRetries(retries int) ClientOption {
	return func(c *Client) {
		c.retries = retries
	}
}

// WithCacheTTL sets the TTL for cached results
func WithCacheTTL(ttl time.Duration) ClientOption {
	return func(c *Client) {
		c.cacheTTL = ttl
	}
}

// WithRateLimit sets the rate limit for API requests
func WithRateLimit(rps float64) ClientOption {
	return func(c *Client) {
		c.limiter = rate.NewLimiter(rate.Limit(rps), int(rps*2))
	}
}

// WithMaxParallel sets the maximum number of parallel requests
func WithMaxParallel(max int) ClientOption {
	return func(c *Client) {
		c.maxParallel = max
	}
}

// NewClient creates a new Lighthouse client
func NewClient(baseURL, apiKey string, options ...ClientOption) *Client {
	client := &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		httpClient:  &http.Client{Timeout: DefaultTimeout},
		retries:     DefaultRetries,
		cacheTTL:    DefaultCacheTTL,
		limiter:     rate.NewLimiter(rate.Limit(DefaultRateLimit), DefaultRateLimit*2),
		maxParallel: DefaultMaxParallel,
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	return client
}

// getCacheKey generates a cache key for the URL and options
func getCacheKey(url string, options AuditOptions) string {
	// Create a unique key based on URL and relevant options
	formFactor := string(options.FormFactor)
	categories := ""
	for _, cat := range options.Categories {
		categories += string(cat) + ","
	}

	refTime := ""
	if !options.ReferenceTime.IsZero() {
		refTime = options.ReferenceTime.Format("20060102")
	} else {
		// Use current date for daily caching by default
		refTime = time.Now().Format("20060102")
	}

	return fmt.Sprintf("lighthouse:%s:%s:%s:%s", url, formFactor, categories, refTime)
}

// getFromCache tries to get the audit result from cache
func (c *Client) getFromCache(ctx context.Context, cacheKey string) (*AuditResult, error) {
	if c.redisClient == nil {
		return nil, nil // Skip caching if Redis is not configured
	}

	data, err := c.redisClient.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("redis error: %w", err)
	}

	var result AuditResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached result: %w", err)
	}

	return &result, nil
}

// saveToCache saves the audit result to cache
func (c *Client) saveToCache(ctx context.Context, cacheKey string, result *AuditResult, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil // Skip caching if Redis is not configured
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	return c.redisClient.Set(ctx, cacheKey, data, ttl).Err()
}

// buildRequestURL builds the request URL for the Lighthouse API
func (c *Client) buildRequestURL(targetURL string, options AuditOptions) (string, error) {
	apiURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	q := apiURL.Query()
	q.Set("url", targetURL)

	// Add categories
	categories := ""
	for _, cat := range options.Categories {
		if categories != "" {
			categories += ","
		}
		categories += string(cat)
	}
	if categories != "" {
		q.Set("category", categories)
	}

	// Add form factor/strategy
	if options.FormFactor == FormFactorDesktop {
		q.Set("strategy", "desktop")
	} else {
		q.Set("strategy", "mobile")
	}

	// Add locale
	if options.Locale != "" {
		q.Set("locale", options.Locale)
	}

	// Add API key
	if c.apiKey != "" {
		q.Set("key", c.apiKey)
	}

	apiURL.RawQuery = q.Encode()
	return apiURL.String(), nil
}

// buildRequestBody builds the request body for the Lighthouse API (for POST requests)
func (c *Client) buildRequestBody(targetURL string, options AuditOptions) ([]byte, error) {
	// Create request body for APIs that use POST (like newer versions of PSI)
	requestBody := map[string]interface{}{
		"url": targetURL,
	}

	// Convert categories to string slice
	categoryStrs := make([]string, len(options.Categories))
	for i, cat := range options.Categories {
		categoryStrs[i] = string(cat)
	}
	requestBody["categories"] = categoryStrs

	// Add form factor
	if options.FormFactor == FormFactorDesktop {
		requestBody["strategy"] = "desktop"
	} else {
		requestBody["strategy"] = "mobile"
	}

	// Add locale
	if options.Locale != "" {
		requestBody["locale"] = options.Locale
	}

	// Add specific audits to include/exclude
	if len(options.OnlyAudits) > 0 {
		requestBody["onlyAudits"] = options.OnlyAudits
	}

	if len(options.SkipAudits) > 0 {
		requestBody["skipAudits"] = options.SkipAudits
	}

	return json.Marshal(requestBody)
}

// doRequest performs an HTTP request with retries
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var resp *http.Response
	var err error

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Retry logic
	for attempt := 0; attempt <= c.retries; attempt++ {
		var req *http.Request
		if method == http.MethodPost && body != nil {
			req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
		} else {
			req, err = http.NewRequestWithContext(ctx, method, url, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break // Success or client error
		}

		// Handle server errors or network issues
		if resp != nil {
			resp.Body.Close()
		}

		// Exponential backoff
		if attempt < c.retries {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue with retry
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.retries, err)
	}

	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// processLighthouseResponse processes the raw Lighthouse API response
func (c *Client) processLighthouseResponse(data []byte) (*AuditResult, error) {
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(data, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Check if it's an error response
	if errMsg, ok := rawResponse["error"].(map[string]interface{}); ok {
		message := "unknown error"
		if msg, ok := errMsg["message"].(string); ok {
			message = msg
		}
		return nil, errors.New(message)
	}

	// Process Lighthouse result
	result := &AuditResult{
		Scores:          make(map[string]float64),
		Audits:          make(map[string]Audit),
		Issues:          []map[string]interface{}{},
		Recommendations: []string{},
	}

	// Extract lighthouseResult from PageSpeed Insights response
	var lighthouseResult map[string]interface{}

	// Check if this is a PSI response or direct Lighthouse response
	if lhr, ok := rawResponse["lighthouseResult"].(map[string]interface{}); ok {
		lighthouseResult = lhr
	} else {
		lighthouseResult = rawResponse
	}

	// Extract basic information
	if version, ok := lighthouseResult["lighthouseVersion"].(string); ok {
		result.LighthouseVersion = version
	}

	if fetchTime, ok := lighthouseResult["fetchTime"].(string); ok {
		result.FetchTime = fetchTime
	}

	if requestedUrl, ok := lighthouseResult["requestedUrl"].(string); ok {
		result.URL = requestedUrl
	}

	if finalUrl, ok := lighthouseResult["finalUrl"].(string); ok {
		result.FinalURL = finalUrl
	}

	if timestamp, ok := lighthouseResult["analysisUTCTimestamp"].(float64); ok {
		result.TotalAnalysisTime = int(timestamp)
	}

	// Extract category scores
	if categories, ok := lighthouseResult["categories"].(map[string]interface{}); ok {
		for catName, catData := range categories {
			if category, ok := catData.(map[string]interface{}); ok {
				if score, ok := category["score"].(float64); ok {
					result.Scores[catName] = score
				}
			}
		}
	}

	// Extract metrics
	var metrics MetricsResult
	if audits, ok := lighthouseResult["audits"].(map[string]interface{}); ok {
		// Process performance metrics
		metricMap := map[string]*float64{
			"first-contentful-paint":   &metrics.FirstContentfulPaint,
			"largest-contentful-paint": &metrics.LargestContentfulPaint,
			"first-meaningful-paint":   &metrics.FirstMeaningfulPaint,
			"speed-index":              &metrics.SpeedIndex,
			"interactive":              &metrics.TimeToInteractive,
			"total-blocking-time":      &metrics.TotalBlockingTime,
			"cumulative-layout-shift":  &metrics.CumulativeLayoutShift,
			"first-cpu-idle":           &metrics.FirstCPUIdle,
			"max-potential-fid":        &metrics.MaxPotentialFID,
			"server-response-time":     &metrics.ServerResponseTime,
		}

		// Integer metrics
		intMetricMap := map[string]*int{
			"dom-size":                       &metrics.DOMSize,
			"render-blocking-resources-size": &metrics.RenderBlockingResources,
			"network-requests":               &metrics.NetworkRequests,
			"total-byte-weight":              &metrics.TotalByteWeight,
		}

		for auditName, audit := range audits {
			if auditData, ok := audit.(map[string]interface{}); ok {
				// Extract metric value
				if numericValue, ok := auditData["numericValue"].(float64); ok {
					if metricPtr, exists := metricMap[auditName]; exists {
						*metricPtr = numericValue
					}
				}

				// Process int metrics
				if numericValue, ok := auditData["numericValue"].(float64); ok {
					if intMetricPtr, exists := intMetricMap[auditName]; exists {
						*intMetricPtr = int(numericValue)
					}
				}

				// Create Audit object
				audit := Audit{
					ID:           auditName,
					Title:        getStringProperty(auditData, "title"),
					Description:  getStringProperty(auditData, "description"),
					DisplayValue: getStringProperty(auditData, "displayValue"),
					Details:      getMapProperty(auditData, "details"),
				}

				// Get score if available
				if score, ok := auditData["score"].(float64); ok {
					audit.Score = score
				} else {
					// Handle informative audits that don't have a score
					audit.Score = -1
				}

				// Get scoreDisplayMode
				if mode, ok := auditData["scoreDisplayMode"].(string); ok {
					audit.ScoreDisplayMode = mode
				}

				// Get numericValue if available
				if numValue, ok := auditData["numericValue"].(float64); ok {
					audit.NumericValue = numValue
				}

				// Add to result audits
				result.Audits[auditName] = audit

				// Extract issues and recommendations
				if audit.Score < 0.5 && audit.Score >= 0 {
					// This is a failing audit, create an issue
					issue := map[string]interface{}{
						"type":        "lighthouse_" + auditName,
						"severity":    getSeverityFromScore(audit.Score),
						"description": audit.Title,
						"details":     audit.Description,
						"score":       audit.Score,
					}
					result.Issues = append(result.Issues, issue)

					// Add recommendation
					result.Recommendations = append(
						result.Recommendations,
						fmt.Sprintf("Улучшите '%s': %s", audit.Title, audit.Description),
					)
				}
			}
		}
	}

	result.Metrics = metrics

	return result, nil
}

// getSeverityFromScore determines the severity level based on a score
func getSeverityFromScore(score float64) string {
	if score < 0.3 {
		return "high"
	} else if score < 0.7 {
		return "medium"
	}
	return "low"
}

// getStringProperty safely extracts a string property from a map
func getStringProperty(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

// getMapProperty safely extracts a map property from a map
func getMapProperty(data map[string]interface{}, key string) map[string]interface{} {
	if val, ok := data[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}

// AnalyzeURL performs a Lighthouse analysis on the given URL
func (c *Client) AnalyzeURL(ctx context.Context, url string, options AuditOptions) (*AuditResult, error) {
	// Generate cache key
	cacheKey := getCacheKey(url, options)

	// Try to get from cache first
	cachedResult, err := c.getFromCache(ctx, cacheKey)
	if err != nil {
		// Log cache error but continue
		fmt.Printf("Cache error: %v\n", err)
	} else if cachedResult != nil {
		return cachedResult, nil
	}

	// Process categories in parallel if needed
	if len(options.Categories) > 1 && c.maxParallel > 1 {
		return c.parallelAnalyze(ctx, url, options)
	}

	// For single category or sequential processing
	return c.singleAnalyze(ctx, url, options)
}

// singleAnalyze performs a single Lighthouse analysis
func (c *Client) singleAnalyze(ctx context.Context, targetURL string, options AuditOptions) (*AuditResult, error) {
	// Generate cache key
	cacheKey := getCacheKey(targetURL, options)

	// Build request URL
	requestURL, err := c.buildRequestURL(targetURL, options)
	if err != nil {
		return nil, err
	}

	// Make request
	var respBody []byte

	// Try GET request first
	respBody, err = c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		// If GET fails with a client error, try POST
		if isClientError(err) {
			// Build request body for POST
			reqBody, buildErr := c.buildRequestBody(targetURL, options)
			if buildErr != nil {
				return nil, fmt.Errorf("failed to build request body: %w", buildErr)
			}

			// Make POST request
			respBody, err = c.doRequest(ctx, http.MethodPost, c.baseURL, reqBody)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Process response
	result, err := c.processLighthouseResponse(respBody)
	if err != nil {
		return nil, err
	}

	// Cache result
	if err := c.saveToCache(ctx, cacheKey, result, options.CacheTTL); err != nil {
		// Log cache error but continue
		fmt.Printf("Failed to cache result: %v\n", err)
	}

	return result, nil
}

// isClientError checks if the error is due to a client error (4xx)
func isClientError(err error) bool {
	return err != nil && (err.Error() == "400" ||
		err.Error() == "401" ||
		err.Error() == "403" ||
		err.Error() == "404")
}

// parallelAnalyze runs multiple Lighthouse analyses in parallel
func (c *Client) parallelAnalyze(ctx context.Context, targetURL string, options AuditOptions) (*AuditResult, error) {
	// Prepare result
	combinedResult := &AuditResult{
		URL:               targetURL,
		LighthouseVersion: "",
		FetchTime:         time.Now().Format(time.RFC3339),
		TotalAnalysisTime: 0,
		Scores:            make(map[string]float64),
		Audits:            make(map[string]Audit),
		Metrics:           MetricsResult{},
		Issues:            []map[string]interface{}{},
		Recommendations:   []string{},
	}

	// Split categories into chunks
	chunks := splitIntoChunks(options.Categories, c.maxParallel)

	// Process each chunk in parallel
	for _, chunk := range chunks {
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]*AuditResult, len(chunk))
		errors := make([]error, len(chunk))

		for i, category := range chunk {
			wg.Add(1)
			go func(idx int, cat Category) {
				defer wg.Done()

				// Create a copy of options with just this category
				singleOptions := options
				singleOptions.Categories = []Category{cat}

				// Run analysis
				result, err := c.singleAnalyze(ctx, targetURL, singleOptions)

				mu.Lock()
				results[idx] = result
				errors[idx] = err
				mu.Unlock()
			}(i, category)
		}

		wg.Wait()

		// Process results from this chunk
		for i, result := range results {
			if errors[i] != nil {
				return nil, fmt.Errorf("error analyzing category %s: %w", chunk[i], errors[i])
			}

			// Merge results
			for k, v := range result.Scores {
				combinedResult.Scores[k] = v
			}

			for k, v := range result.Audits {
				combinedResult.Audits[k] = v
			}

			// Take the metrics from performance category
			if chunk[i] == CategoryPerformance {
				combinedResult.Metrics = result.Metrics
			}

			// Combine issues and recommendations
			combinedResult.Issues = append(combinedResult.Issues, result.Issues...)
			combinedResult.Recommendations = append(combinedResult.Recommendations, result.Recommendations...)

			// Take the latest lighthouse version
			if result.LighthouseVersion != "" {
				combinedResult.LighthouseVersion = result.LighthouseVersion
			}
		}
	}

	// Cache combined result
	cacheKey := getCacheKey(targetURL, options)
	if err := c.saveToCache(ctx, cacheKey, combinedResult, options.CacheTTL); err != nil {
		// Log cache error but continue
		fmt.Printf("Failed to cache combined result: %v\n", err)
	}

	return combinedResult, nil
}

// splitIntoChunks splits a slice into chunks of the given size
func splitIntoChunks(slice []Category, chunkSize int) [][]Category {
	var chunks [][]Category
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// SchedulePeriodicUpdates schedules periodic updates for a list of URLs
func (c *Client) SchedulePeriodicUpdates(urls []string, options AuditOptions, interval time.Duration, callback func(string, *AuditResult, error)) (context.CancelFunc, error) {
	if interval < time.Minute {
		return nil, errors.New("interval must be at least 1 minute")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			// Process all URLs
			for _, url := range urls {
				select {
				case <-ctx.Done():
					return
				default:
					// Set reference time to current time
					options.ReferenceTime = time.Now()

					// Run analysis
					result, err := c.AnalyzeURL(ctx, url, options)

					// Call callback with result
					if callback != nil {
						callback(url, result, err)
					}

					// Sleep a bit between URLs to avoid rate limiting
					time.Sleep(time.Second)
				}
			}

			// Wait for next interval
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel, nil
}

// GetLatestAnalysis gets the latest analysis for a URL
func (c *Client) GetLatestAnalysis(ctx context.Context, url string) (*AuditResult, error) {
	if c.redisClient == nil {
		return nil, errors.New("redis client not configured")
	}

	// Look for any keys matching this URL
	pattern := fmt.Sprintf("lighthouse:%s:*", url)
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to search for keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, errors.New("no analysis found for URL")
	}

	// Sort keys to get the most recent one (by date in the key)
	// In a production system, you'd want to parse the keys and sort by date
	// For simplicity, we'll just use the first key here

	data, err := c.redisClient.Get(ctx, keys[0]).Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached result: %w", err)
	}

	var result AuditResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached result: %w", err)
	}

	return &result, nil
}

// ForceRefreshAnalysis forces a refresh of the analysis for a URL
func (c *Client) ForceRefreshAnalysis(ctx context.Context, url string, options AuditOptions) (*AuditResult, error) {
	// Generate cache key
	cacheKey := getCacheKey(url, options)

	// Delete existing cache entry
	if c.redisClient != nil {
		if err := c.redisClient.Del(ctx, cacheKey).Err(); err != nil {
			// Log but continue
			fmt.Printf("Failed to delete cache key: %v\n", err)
		}
	}

	// Run new analysis
	return c.AnalyzeURL(ctx, url, options)
}

// GetAnalysisHistoryKeys gets all cache keys for a URL's analysis history
func (c *Client) GetAnalysisHistoryKeys(ctx context.Context, url string) ([]string, error) {
	if c.redisClient == nil {
		return nil, errors.New("redis client not configured")
	}

	// Look for any keys matching this URL
	pattern := fmt.Sprintf("lighthouse:%s:*", url)
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to search for keys: %w", err)
	}

	return keys, nil
}
