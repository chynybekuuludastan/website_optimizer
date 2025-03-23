package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AnalysisHandler обрабатывает запросы по анализу веб-сайтов
type AnalysisHandler struct {
	AnalysisRepo       repository.AnalysisRepository
	WebsiteRepo        repository.WebsiteRepository
	MetricsRepo        repository.MetricsRepository
	IssueRepo          repository.IssueRepository
	RecommendationRepo repository.RecommendationRepository
	RedisClient        *database.RedisClient
	Config             *config.Config
	cancelFunctions    sync.Map
}

func NewAnalysisHandler(
	repoFactory *repository.Factory,
	redisClient *database.RedisClient,
	cfg *config.Config,
) *AnalysisHandler {
	return &AnalysisHandler{
		AnalysisRepo:       repoFactory.AnalysisRepository,
		WebsiteRepo:        repoFactory.WebsiteRepository,
		MetricsRepo:        repoFactory.MetricsRepository,
		IssueRepo:          repoFactory.IssueRepository,
		RecommendationRepo: repoFactory.RecommendationRepository,
		RedisClient:        redisClient,
		Config:             cfg,
		cancelFunctions:    sync.Map{},
	}
}

// @Summary Create a new website analysis
// @Description Starts an analysis of the provided website URL
// @Tags analysis
// @Accept json
// @Produce json
// @Param analysis body AnalysisRequest true "Analysis Request"
// @Success 201 {object} map[string]interface{} "Analysis created successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis [post]
func (h *AnalysisHandler) CreateAnalysis(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	req := new(AnalysisRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Создаем или получаем веб-сайт
	website, err := h.WebsiteRepo.FindByURL(req.URL)
	if err != nil {
		// Веб-сайт не найден, создаем новый
		website = &models.Website{
			URL: req.URL,
		}
		if err := h.WebsiteRepo.Create(website); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to create website record: " + err.Error(),
			})
		}
	}

	// Создаем анализ
	analysis := models.Analysis{
		WebsiteID: website.ID,
		UserID:    userID,
		Status:    "pending",
		StartedAt: time.Now(),
	}

	if err := h.AnalysisRepo.Create(&analysis); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create analysis record: " + err.Error(),
		})
	}

	// Запускаем анализ в фоновом режиме
	go h.runAnalysis(analysis.ID, req.URL)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"analysis_id": analysis.ID,
			"status":      analysis.Status,
		},
	})
}

func (a *AnalysisHandler) runAnalysis(analysisID uuid.UUID, url string) {
	// Update analysis status in database
	if err := a.AnalysisRepo.UpdateStatus(analysisID, "running"); err != nil {
		a.updateAnalysisFailed(analysisID, "Error updating status: "+err.Error())
		return
	}

	// Ensure timeout is within reasonable limits (max 5 minutes)
	timeout := a.Config.AnalysisTimeout
	if timeout <= 0 || timeout > 300 {
		timeout = 300
	}

	// Create cancellable context with reasonable timeout
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(timeout)*time.Second,
	)
	defer cancel()

	// Store the cancel function in a registry to allow cancellation via WebSocket
	a.cancelFunctions.Store(analysisID.String(), cancel)
	defer a.cancelFunctions.Delete(analysisID.String())

	// Parse the website with resource limits and context for cancellation
	websiteData, err := parser.ParseWebsite(url, parser.ParseOptions{
		Timeout: timeout,
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Parsing error: "+err.Error())
		return
	}

	// Check if context is done (cancelled or timed out)
	select {
	case <-ctx.Done():
		a.updateAnalysisFailed(analysisID, "Analysis cancelled or timed out: "+ctx.Err().Error())
		return
	default:
		// Continue with analysis
	}

	// Update website information
	website := &models.Website{
		ID:          analysisID,
		Title:       websiteData.Title,
		Description: websiteData.Description,
	}
	if err := a.WebsiteRepo.Update(website); err != nil {
		a.updateAnalysisFailed(analysisID, "Error updating website info: "+err.Error())
		return
	}

	// Create analyzer manager with progress tracking - register only essential analyzers
	manager := analyzer.NewAnalyzerManager()

	// Register only critical analyzers to reduce processing time
	manager.RegisterCriticalAnalyzers()

	// Register progress callback with rate limiting
	progressChan := make(chan analyzer.ProgressUpdate, 20) // Buffered channel
	lastProgressTime := time.Now()
	manager.SetProgressCallback(func(update analyzer.ProgressUpdate) {
		// Rate limit progress updates to reduce WebSocket traffic
		now := time.Now()
		if math.Mod(update.Progress, 10) == 0 || update.Progress == 100 || now.Sub(lastProgressTime) > time.Second {
			select {
			case progressChan <- update:
				lastProgressTime = now
			default:
				// Skip update if channel is full (non-blocking)
			}
		}
	})

	// Run the analyzers with timeout
	results, err := manager.RunAllAnalyzers(ctx, websiteData)

	// Check if the analysis was cancelled or timed out
	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			a.AnalysisRepo.UpdateStatus(analysisID, "cancelled")
			return
		} else if ctx.Err() == context.DeadlineExceeded {
			// Analysis timed out
			a.updateAnalysisFailed(analysisID, "Analysis timed out")
			return
		}
	default:
		// Analysis completed or failed normally
	}

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Analysis error: "+err.Error())
		return
	}

	// Split database operations into separate transactions to avoid long locks
	// First transaction: save metrics
	err = a.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		totalMetrics := 0
		for analyzerType, result := range results {
			// Only save essential metrics (score and basic info)
			essentialData := map[string]interface{}{
				"score": result["score"],
				"type":  string(analyzerType),
			}

			metricData, err := json.Marshal(essentialData)
			if err != nil {
				return fmt.Errorf("error serializing results: %w", err)
			}

			metric := models.AnalysisMetric{
				AnalysisID: analysisID,
				Category:   string(analyzerType),
				Name:       string(analyzerType) + "_score",
				Value:      datatypes.JSON(metricData),
			}
			if err := tx.Create(&metric).Error; err != nil {
				return fmt.Errorf("error saving metric: %w", err)
			}
			totalMetrics++

		}
		return nil
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Error saving metrics: "+err.Error())
		return
	}

	err = a.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		allIssues := manager.GetAllIssues()

		for analyzerType, issues := range allIssues {
			// Limit to 10 most important issues per category
			maxIssues := 10
			if len(issues) > maxIssues {
				// Sort issues by severity (high first)
				sort.Slice(issues, func(i, j int) bool {
					sevI, _ := issues[i]["severity"].(string)
					sevJ, _ := issues[j]["severity"].(string)
					return getSeverityValue(sevI) > getSeverityValue(sevJ)
				})
				issues = issues[:maxIssues]
			}

			// Save each issue
			for _, issue := range issues {
				severity := issue["severity"].(string)
				description := issue["description"].(string)

				issueRecord := models.Issue{
					AnalysisID:  analysisID,
					Category:    string(analyzerType),
					Severity:    severity,
					Title:       description,
					Description: description,
				}

				if location, ok := issue["url"].(string); ok {
					issueRecord.Location = location
				} else if count, ok := issue["count"].(int); ok {
					issueRecord.Location = fmt.Sprintf("Count: %d", count)
				}

				if err := tx.Create(&issueRecord).Error; err != nil {
					return fmt.Errorf("error saving issue: %w", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Error saving issues: "+err.Error())
		return
	}

	err = a.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		allRecommendations := manager.GetAllRecommendations()
		uniqueRecommendations := make(map[string]struct{})
		totalRecs := 0
		maxRecs := 20 // Maximum 20 recommendations total

		for analyzerType, recommendations := range allRecommendations {
			for _, rec := range recommendations {
				// Skip duplicate recommendations
				if _, ok := uniqueRecommendations[rec]; ok {
					continue
				}
				uniqueRecommendations[rec] = struct{}{}

				// Stop after reaching maximum recommendations
				if totalRecs >= maxRecs {
					break
				}
				totalRecs++

				// Determine priority
				priority := "medium" // Default value
				switch analyzerType {
				case analyzer.SEOType, analyzer.SecurityType:
					priority = "high"
				case analyzer.PerformanceType, analyzer.AccessibilityType:
					priority = "medium"
				case analyzer.StructureType, analyzer.MobileType, analyzer.ContentType:
					priority = "low"
				}

				recommendation := models.Recommendation{
					AnalysisID:  analysisID,
					Category:    string(analyzerType),
					Priority:    priority,
					Title:       rec,
					Description: rec,
				}

				if err := tx.Create(&recommendation).Error; err != nil {
					return fmt.Errorf("error saving recommendation: %w", err)
				}
			}

			// Break outer loop if we've reached maximum
			if totalRecs >= maxRecs {
				break
			}
		}

		return nil
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Error saving recommendations: "+err.Error())
		return
	}

	// Update analysis to completed status
	if err := a.AnalysisRepo.UpdateStatus(analysisID, "completed"); err != nil {
		a.updateAnalysisFailed(analysisID, "Error updating completion status: "+err.Error())
		return
	}

	// Calculate overall score
	var totalScore float64
	var scoreCount int
	for _, result := range results {
		if score, ok := result["score"].(float64); ok {
			totalScore += score
			scoreCount++
		}
	}
}

// Helper function to get numeric value for severity to sort issues
func getSeverityValue(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// updateAnalysisFailed handles analysis failure
func (a *AnalysisHandler) updateAnalysisFailed(analysisID uuid.UUID, errorMsg string) {
	// Create metadata with error
	metadata := datatypes.JSON([]byte(`{"error": "` + errorMsg + `"}`))

	// Update the database
	a.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		analysis := &models.Analysis{
			ID:          analysisID,
			Status:      "failed",
			CompletedAt: time.Now(),
			Metadata:    metadata,
		}
		return tx.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(analysis).Error
	})
}

// @Summary Get overall analysis score
// @Description Calculates and returns the overall score for all analysis categories
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Success 200 {object} map[string]interface{} "Overall score data"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 404 {object} map[string]interface{} "Analysis not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/score [get]
func (h *AnalysisHandler) GetOverallScore(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Create a cache key for this score
	cacheKey := "analysis_score:" + analysisID.String()

	// Try to get from cache if Redis is available
	if h.RedisClient != nil {
		var scoreData map[string]interface{}
		err := h.RedisClient.Get(cacheKey, &scoreData)
		if err == nil && scoreData != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    scoreData,
				"cached":  true,
			})
		}
	}

	// Get all metrics for the analysis
	metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get metrics",
		})
	}

	// Calculate overall score based on all categories
	totalScore := 0.0
	count := 0
	categoryScores := make(map[string]float64)

	for _, metric := range metrics {
		var result map[string]interface{}
		if err := json.Unmarshal(metric.Value, &result); err != nil {
			continue
		}

		if score, ok := result["score"].(float64); ok {
			totalScore += score
			count++
			categoryScores[metric.Category] = score
		}
	}

	// Calculate average score
	averageScore := 0.0
	if count > 0 {
		averageScore = totalScore / float64(count)
	}

	// Structure the response
	scoreData := fiber.Map{
		"overall_score":   averageScore,
		"category_count":  count,
		"category_scores": categoryScores,
	}

	// Cache the result if Redis is available
	if h.RedisClient != nil {
		h.RedisClient.Set(cacheKey, scoreData, 30*time.Minute) // Cache for 30 minutes
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    scoreData,
	})
}

// @Summary Get analysis category summary
// @Description Returns a summary of analysis results for a specific category
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Param category path string true "Analysis category (seo, performance, structure, etc.)"
// @Success 200 {object} map[string]interface{} "Category summary data"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 404 {object} map[string]interface{} "Metrics not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/summary/{category} [get]
func (h *AnalysisHandler) GetCategorySummary(c *fiber.Ctx) error {
	id := c.Params("id")
	category := c.Params("category")

	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Недопустимый ID анализа",
		})
	}

	// First, check if the analysis exists and get its status
	var analysis models.Analysis
	err = h.AnalysisRepo.FindByID(analysisID, &analysis)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Анализ не найден",
		})
	}

	// If analysis failed, get error from metadata and return it
	if analysis.Status == "failed" {
		var metadata map[string]interface{}
		if err := json.Unmarshal(analysis.Metadata, &metadata); err == nil {
			if errMsg, ok := metadata["error"].(string); ok {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   "Анализ завершился с ошибкой",
					"details": errMsg,
					"status":  analysis.Status,
				})
			}
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Анализ завершился с ошибкой",
			"status":  analysis.Status,
		})
	}

	// Get metrics for this category
	metrics, err := h.MetricsRepo.FindByCategory(analysisID, category)
	if err != nil || len(metrics) == 0 {
		// Instead of 404, return a more informative response
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"category":              category,
				"status":                analysis.Status,
				"metrics":               nil,
				"issues":                []interface{}{},
				"recommendations":       []interface{}{},
				"issues_count":          0,
				"recommendations_count": 0,
			},
		})
	}

	// Get issues for this category
	issues, err := h.IssueRepo.FindByCategory(analysisID, category)
	if err != nil {
		issues = []models.Issue{}
	}

	// Get recommendations for this category
	recommendations, err := h.RecommendationRepo.FindByCategory(analysisID, category)
	if err != nil {
		recommendations = []models.Recommendation{}
	}

	// Создаем оптимизированные структуры для ответа
	type OptimizedIssue struct {
		ID          string    `json:"id"`
		Category    string    `json:"category"`
		Severity    string    `json:"severity"`
		Title       string    `json:"title"`
		Description string    `json:"description,omitempty"`
		Location    string    `json:"location,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
	}

	type OptimizedRecommendation struct {
		ID          string    `json:"id"`
		Category    string    `json:"category"`
		Priority    string    `json:"priority"`
		Title       string    `json:"title"`
		Description string    `json:"description,omitempty"`
		CodeSnippet string    `json:"code_snippet,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
	}

	// Преобразуем проблемы в оптимизированный формат
	optimizedIssues := make([]OptimizedIssue, len(issues))
	for i, issue := range issues {
		optimizedIssues[i] = OptimizedIssue{
			ID:          issue.ID.String(),
			Category:    issue.Category,
			Severity:    issue.Severity,
			Title:       issue.Title,
			Description: issue.Description,
			Location:    issue.Location,
			CreatedAt:   issue.CreatedAt,
		}
	}

	// Преобразуем рекомендации в оптимизированный формат
	optimizedRecommendations := make([]OptimizedRecommendation, len(recommendations))
	for i, rec := range recommendations {
		optimizedRecommendations[i] = OptimizedRecommendation{
			ID:          rec.ID.String(),
			Category:    rec.Category,
			Priority:    rec.Priority,
			Title:       rec.Title,
			Description: rec.Description,
			CodeSnippet: rec.CodeSnippet,
			CreatedAt:   rec.CreatedAt,
		}
	}

	// Обрабатываем метрики
	var result map[string]interface{}
	if err := json.Unmarshal(metrics[0].Value, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Не удалось обработать метрики: " + err.Error(),
		})
	}

	// Формируем финальный оптимизированный ответ
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"metrics":               result,
			"issues":                optimizedIssues,
			"recommendations":       optimizedRecommendations,
			"score":                 result["score"],
			"issues_count":          len(optimizedIssues),
			"recommendations_count": len(optimizedRecommendations),
			"status":                analysis.Status,
			"website_url":           analysis.Website.URL, // Добавляем полезную информацию
			"analysis_id":           analysisID.String(),  // ID анализа для дальнейших запросов
		},
	})
}

// @Summary Get user's latest analyses
// @Description Returns the most recent analyses for the current user
// @Tags analysis
// @Accept json
// @Produce json
// @Param limit query int false "Number of analyses to return" default(5)
// @Success 200 {object} map[string]interface{} "Latest analyses"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/latest [get]
func (h *AnalysisHandler) GetLatestAnalyses(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	// Parse limit parameter with default value
	limit := 5
	if c.Query("limit") != "" {
		if limitParam, err := strconv.Atoi(c.Query("limit")); err == nil && limitParam > 0 {
			limit = limitParam
		}
	}

	// Use the new repository method with caching
	analyses, err := h.AnalysisRepo.FindLatestByUserID(userID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch latest analyses: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    analyses,
	})
}

// @Summary Get analysis statistics
// @Description Returns analysis statistics by date range and status
// @Tags analysis
// @Accept json
// @Produce json
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Param status query string false "Analysis status (pending, running, completed, failed)"
// @Success 200 {object} map[string]interface{} "Analysis statistics"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/statistics [get]
func (h *AnalysisHandler) GetAnalyticsStatistics(c *fiber.Ctx) error {
	// Parse date range
	startDate := time.Now().AddDate(0, -1, 0) // Default: 1 month ago
	endDate := time.Now()

	if c.Query("start_date") != "" {
		if parsedDate, err := time.Parse("2006-01-02", c.Query("start_date")); err == nil {
			startDate = parsedDate
		}
	}

	if c.Query("end_date") != "" {
		if parsedDate, err := time.Parse("2006-01-02", c.Query("end_date")); err == nil {
			endDate = parsedDate.AddDate(0, 0, 1) // Include the end date
		}
	}

	// Get status parameter (optional)
	status := c.Query("status")

	// Use the new CountByStatusAndDate method
	count, err := h.AnalysisRepo.CountByStatusAndDate(status, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get analysis statistics: " + err.Error(),
		})
	}

	// Build a more comprehensive response with time periods
	statsResponse := map[string]interface{}{
		"total":      count,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	}

	if status != "" {
		statsResponse["status"] = status
	}

	// If RedisClient is available, we could also cache these statistics
	if h.RedisClient != nil {
		cacheKey := fmt.Sprintf("stats:%s:%s:%s",
			startDate.Format("2006-01-02"),
			endDate.Format("2006-01-02"),
			status)

		// Cache for 1 hour
		h.RedisClient.Set(cacheKey, statsResponse, time.Hour)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    statsResponse,
	})
}

// @Summary Update analysis metadata
// @Description Updates the metadata for a specific analysis
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Param metadata body map[string]interface{} true "Metadata to update"
// @Success 200 {object} map[string]interface{} "Metadata updated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Analysis not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/metadata [patch]
func (h *AnalysisHandler) UpdateMetadata(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Parse request body
	var metadataRequest map[string]interface{}
	if err := c.BodyParser(&metadataRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Verify that the analysis exists
	var analysis models.Analysis
	if err := h.AnalysisRepo.FindByID(analysisID, &analysis); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Get current metadata (if any)
	var currentMetadata map[string]interface{}
	if analysis.Metadata != nil {
		if err := json.Unmarshal(analysis.Metadata, &currentMetadata); err == nil {
			// Merge existing metadata with new metadata
			for k, v := range metadataRequest {
				currentMetadata[k] = v
			}
		} else {
			// If we can't unmarshal existing metadata, use only the new metadata
			currentMetadata = metadataRequest
		}
	} else {
		// No existing metadata
		currentMetadata = metadataRequest
	}

	// Convert back to JSON
	metadataJSON, err := json.Marshal(currentMetadata)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to marshal metadata: " + err.Error(),
		})
	}

	// Update analysis metadata using the new repository method
	if err := h.AnalysisRepo.UpdateMetadata(analysisID, datatypes.JSON(metadataJSON)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update metadata: " + err.Error(),
		})
	}

	// If caching is available, invalidate any cached analysis data
	if h.RedisClient != nil {
		cacheKey := "analysis_score:" + analysisID.String()
		h.RedisClient.Delete(cacheKey)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Metadata updated successfully",
		"data": fiber.Map{
			"metadata": currentMetadata,
		},
	})
}

// AnalysisRequest представляет запрос на анализ веб-сайта
type AnalysisRequest struct {
	URL string `json:"url" validate:"required,url"`
}
