package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
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

type AnalysisRequest struct {
	URL string `json:"url" validate:"required,url"`
}

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

// GetAnalysisMetrics returns all metrics for a specific analysis
// @Summary Get all metrics for an analysis
// @Description Returns all metrics for a specific analysis
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Success 200 {object} map[string]interface{} "Analysis metrics"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Analysis not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/metrics [get]
func (h *AnalysisHandler) GetAnalysisMetrics(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Check if analysis exists
	var analysis models.Analysis
	if err := h.AnalysisRepo.FindByID(analysisID, &analysis); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Create a cache key for this request
	cacheKey := "analysis_metrics:" + analysisID.String()

	// Try to get from cache if Redis is available
	if h.RedisClient != nil {
		var cachedMetrics []map[string]interface{}
		err := h.RedisClient.Get(cacheKey, &cachedMetrics)
		if err == nil && cachedMetrics != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    cachedMetrics,
				"cached":  true,
			})
		}
	}

	// Get all metrics for this analysis
	metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch metrics",
		})
	}

	// Process metrics to make them more usable in the frontend
	formattedMetrics := make([]map[string]interface{}, 0, len(metrics))
	for _, metric := range metrics {
		var value map[string]interface{}
		if err := json.Unmarshal(metric.Value, &value); err != nil {
			continue
		}

		formattedMetric := map[string]interface{}{
			"id":        metric.ID,
			"category":  metric.Category,
			"name":      metric.Name,
			"value":     value,
			"createdAt": metric.CreatedAt,
		}
		formattedMetrics = append(formattedMetrics, formattedMetric)
	}

	// Cache the result if Redis is available
	if h.RedisClient != nil {
		h.RedisClient.Set(cacheKey, formattedMetrics, 30*time.Minute) // Cache for 30 minutes
	}

	return c.JSON(formattedMetrics)
}

// GetAnalysisMetricsByCategory returns metrics for a specific category
// @Summary Get metrics by category
// @Description Returns metrics for a specific analysis category
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Param category path string true "Metric category"
// @Success 200 {object} map[string]interface{} "Metrics for the specified category"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Analysis or category not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/metrics/{category} [get]
func (h *AnalysisHandler) GetAnalysisMetricsByCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	category := c.Params("category")

	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Check if analysis exists
	var analysis models.Analysis
	if err := h.AnalysisRepo.FindByID(analysisID, &analysis); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Create a cache key for this request
	cacheKey := fmt.Sprintf("analysis_metrics:%s:%s", analysisID.String(), category)

	// Try to get from cache if Redis is available
	if h.RedisClient != nil {
		var cachedMetrics []map[string]interface{}
		err := h.RedisClient.Get(cacheKey, &cachedMetrics)
		if err == nil && cachedMetrics != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    cachedMetrics,
				"cached":  true,
			})
		}
	}

	// Get metrics for this category
	metrics, err := h.MetricsRepo.FindByCategory(analysisID, category)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch metrics",
		})
	}

	if len(metrics) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "No metrics found for this category",
		})
	}

	// Process metrics to make them more usable in the frontend
	formattedMetrics := make([]map[string]interface{}, 0, len(metrics))
	for _, metric := range metrics {
		var value map[string]interface{}
		if err := json.Unmarshal(metric.Value, &value); err != nil {
			continue
		}

		formattedMetric := map[string]interface{}{
			"id":        metric.ID,
			"category":  metric.Category,
			"name":      metric.Name,
			"value":     value,
			"createdAt": metric.CreatedAt,
		}
		formattedMetrics = append(formattedMetrics, formattedMetric)
	}

	// Cache the result if Redis is available
	if h.RedisClient != nil {
		h.RedisClient.Set(cacheKey, formattedMetrics, 30*time.Minute) // Cache for 30 minutes
	}

	return c.JSON(formattedMetrics)
}

// GetAnalysisIssues returns all issues found during analysis
// @Summary Get issues for an analysis
// @Description Returns all issues found during analysis
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Success 200 {object} map[string]interface{} "Analysis issues"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Analysis not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /analysis/{id}/issues [get]
func (h *AnalysisHandler) GetAnalysisIssues(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Check if analysis exists
	var analysis models.Analysis
	if err := h.AnalysisRepo.FindByID(analysisID, &analysis); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Create a cache key for this request
	cacheKey := "analysis_issues:" + analysisID.String()

	// Try to get from cache if Redis is available
	if h.RedisClient != nil {
		var cachedIssues []models.Issue
		err := h.RedisClient.Get(cacheKey, &cachedIssues)
		if err == nil && cachedIssues != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    cachedIssues,
				"cached":  true,
			})
		}
	}

	// Get all issues for this analysis
	issues, err := h.IssueRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch issues",
		})
	}

	// Cache the result if Redis is available
	if h.RedisClient != nil {
		h.RedisClient.Set(cacheKey, issues, 30*time.Minute) // Cache for 30 minutes
	}

	return c.JSON(issues)
}

func (a *AnalysisHandler) runAnalysis(analysisID uuid.UUID, url string) {
	if err := a.AnalysisRepo.UpdateStatus(analysisID, "running"); err != nil {
		a.updateAnalysisFailed(analysisID, "Error updating status: "+err.Error())
		return
	}

	timeout := a.Config.AnalysisTimeout
	if timeout <= 0 || timeout > 300 {
		timeout = 300
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(timeout)*time.Second,
	)
	defer cancel()

	a.cancelFunctions.Store(analysisID.String(), cancel)
	defer a.cancelFunctions.Delete(analysisID.String())

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

func (a *AnalysisHandler) updateAnalysisFailed(analysisID uuid.UUID, errorMsg string) {
	metadata := datatypes.JSON([]byte(`{"error": "` + errorMsg + `"}`))

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
