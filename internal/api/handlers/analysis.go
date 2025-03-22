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
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"

	ws "github.com/chynybekuuludastan/website_optimizer/internal/api/websocket"
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
	WebSocketHub       *ws.Hub
	cancelFunctions    sync.Map
}

func NewAnalysisHandler(
	repoFactory *repository.Factory,
	redisClient *database.RedisClient,
	cfg *config.Config,
	wsHub *ws.Hub, // Add this parameter
) *AnalysisHandler {
	return &AnalysisHandler{
		AnalysisRepo:       repoFactory.AnalysisRepository,
		WebsiteRepo:        repoFactory.WebsiteRepository,
		MetricsRepo:        repoFactory.MetricsRepository,
		IssueRepo:          repoFactory.IssueRepository,
		RecommendationRepo: repoFactory.RecommendationRepository,
		RedisClient:        redisClient,
		Config:             cfg,
		WebSocketHub:       wsHub, // Initialize the field
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
	// Report analysis started
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisStarted,
		Status:    "started",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"analysis_id": analysisID.String(),
			"url":         url,
			"message":     "Analysis started",
		},
	})

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

	// Report parsing started
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "parsing",
		Progress:  5.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "parsing",
			"message":  "Parsing website content",
			"progress": 5.0,
		},
	})

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

	// Report parsing completed and analysis beginning
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "analyzing",
		Progress:  15.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "parsing_completed",
			"message":  "Website parsed successfully, starting analysis",
			"progress": 15.0,
			"details": map[string]interface{}{
				"title":       websiteData.Title,
				"description": websiteData.Description,
				"images":      len(websiteData.Images),
				"links":       len(websiteData.Links),
				"scripts":     len(websiteData.Scripts),
				"styles":      len(websiteData.Styles),
			},
		},
	})

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

	// Start progress reporting goroutine
	go func() {
		defer close(progressChan)
		for update := range progressChan {
			// Check if context is done before sending progress
			select {
			case <-ctx.Done():
				return
			default:
				// Calculate overall progress (15-85% range for analyzers)
				progress := 15.0 + (update.Progress * 70.0 / 100.0)

				// Send more compact progress updates
				a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
					Type:      ws.TypeAnalysisProgress,
					Status:    "analyzing",
					Progress:  progress,
					Category:  ws.AnalysisCategory(update.AnalyzerType),
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"analyzer":         update.AnalyzerType,
						"overall_progress": progress,
						"message":          update.Message,
					},
				})

				// Only send partial results when significant
				if update.PartialResults != nil && len(update.PartialResults) > 0 {
					a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
						Type:      ws.TypePartialResults,
						Category:  ws.AnalysisCategory(update.AnalyzerType),
						Timestamp: time.Now(),
						Data: map[string]interface{}{
							"analyzer": update.AnalyzerType,
							"results":  update.PartialResults,
						},
					})
				}
			}
		}
	}()

	// Run the analyzers with timeout
	results, err := manager.RunAllAnalyzers(ctx, websiteData)

	// Check if the analysis was cancelled or timed out
	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			// Analysis was cancelled
			a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
				Type:      ws.TypeAnalysisCompleted,
				Status:    "cancelled",
				Progress:  100.0,
				Category:  ws.CategoryAll,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"message": "Analysis was cancelled",
				},
			})
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

	// Report that we're saving results
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "saving",
		Progress:  85.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "saving_results",
			"message":  "Saving analysis results",
			"progress": 85.0,
		},
	})

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

			// Report progress every 2 metrics
			if totalMetrics%2 == 0 {
				progress := 85.0 + (float64(totalMetrics) / float64(len(results)) * 5.0)
				a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
					Type:      ws.TypeAnalysisProgress,
					Status:    "saving",
					Progress:  progress,
					Category:  ws.CategoryAll,
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"stage":    "saving_metrics",
						"progress": progress,
					},
				})
			}
		}
		return nil
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Error saving metrics: "+err.Error())
		return
	}

	// Second transaction: save critical issues (max 10 per category)
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "saving",
		Progress:  90.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "saving_issues",
			"message":  "Saving identified issues",
			"progress": 90.0,
		},
	})

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

	// Third transaction: save recommendations (up to 20 total)
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "saving",
		Progress:  95.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "saving_recommendations",
			"message":  "Saving recommendations",
			"progress": 95.0,
		},
	})

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

	overallScore := 0.0
	if scoreCount > 0 {
		overallScore = totalScore / float64(scoreCount)
	}

	startTime := time.Now()

	// Send completion message
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisCompleted,
		Status:    "completed",
		Progress:  100.0,
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message":        "Analysis completed successfully",
			"overall_score":  overallScore,
			"analyzer_count": scoreCount,
			"duration_ms":    time.Since(startTime).Milliseconds(),
		},
	})
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

	// Send error notification via WebSocket
	a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisError,
		Status:    "failed",
		Category:  ws.CategoryAll,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": "Analysis failed",
			"error":   errorMsg,
		},
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
			"error":   "Недопустимый ID анализа",
		})
	}

	// Получаем все метрики для анализа
	metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Не удалось получить метрики",
		})
	}

	// Вычисляем общий балл на основе всех категорий
	totalScore := 0.0
	count := 0

	for _, metric := range metrics {
		var result map[string]interface{}
		if err := json.Unmarshal(metric.Value, &result); err != nil {
			continue
		}

		if score, ok := result["score"].(float64); ok {
			totalScore += score
			count++
		}
	}

	// Вычисляем средний балл
	averageScore := 0.0
	if count > 0 {
		averageScore = totalScore / float64(count)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"overall_score":  averageScore,
			"category_count": count,
		},
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
				"issues":                []models.Issue{},
				"recommendations":       []models.Recommendation{},
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

	// Parse metrics data
	var result map[string]interface{}
	if err := json.Unmarshal(metrics[0].Value, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Не удалось обработать метрики: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"metrics":               result,
			"issues":                issues,
			"recommendations":       recommendations,
			"score":                 result["score"],
			"issues_count":          len(issues),
			"recommendations_count": len(recommendations),
			"status":                analysis.Status,
		},
	})
}

// @Summary Real-time analysis updates
// @Description WebSocket endpoint for receiving real-time analysis status updates
// @Tags analysis
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID"
// @Success 101 {object} nil "Switching protocols to WebSocket"
// @Failure 400 {object} map[string]interface{} "Invalid analysis ID"
// @Failure 404 {object} map[string]interface{} "Analysis not found"
// @Router /ws/analysis/{id} [get]
// HandleWebSocket handles WebSocket connections with improved efficiency and error handling
func (h *AnalysisHandler) HandleWebSocket(c *websocket.Conn) {
	// Получаем ID анализа из URL
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		c.WriteJSON(fiber.Map{
			"success": false,
			"error":   "Недопустимый ID анализа",
		})
		c.Close()
		return
	}

	// Получаем данные анализа
	var analysis models.Analysis
	err = h.AnalysisRepo.FindByID(analysisID, &analysis)
	if err != nil {
		c.WriteJSON(fiber.Map{
			"success": false,
			"error":   "Анализ не найден",
		})
		c.Close()
		return
	}

	// Используем меньший интервал обновлений для снижения нагрузки
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Канал для обработки закрытия соединения
	disconnect := make(chan bool, 1)
	defer close(disconnect)

	// Контекст с отменой для управления горутинами
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Устанавливаем таймаут чтения и обработчик pong
	c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Обработка входящих сообщений (включая закрытие)
	go func() {
		defer func() {
			disconnect <- true
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				messageType, _, err := c.ReadMessage()
				if err != nil || messageType == websocket.CloseMessage {
					return
				}
			}
		}
	}()

	// Отправляем начальное состояние
	initialStatus := fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":           analysis.ID,
			"status":       analysis.Status,
			"completed_at": analysis.CompletedAt,
		},
	}

	if err := c.WriteJSON(initialStatus); err != nil {
		c.Close()
		return
	}

	// Track last update time to prevent sending too many updates
	lastUpdateTime := time.Now()
	var lastStatus string = analysis.Status

	// Основной цикл отправки обновлений
	for {
		select {
		case <-ticker.C:
			// Обновляем данные анализа только если прошло достаточно времени с последнего обновления
			now := time.Now()
			if now.Sub(lastUpdateTime) < 2*time.Second && lastStatus != "completed" && lastStatus != "failed" {
				continue // Skip update if too recent
			}

			err = h.AnalysisRepo.FindByID(analysisID, &analysis)
			if err != nil {
				c.WriteJSON(fiber.Map{
					"success": false,
					"error":   "Ошибка получения анализа",
				})
				c.Close()
				return
			}

			// Отправляем обновление только если статус изменился
			if analysis.Status != lastStatus {
				lastUpdateTime = now
				lastStatus = analysis.Status

				// Отправляем текущий статус клиенту
				if err := c.WriteJSON(fiber.Map{
					"success": true,
					"data": fiber.Map{
						"id":           analysis.ID,
						"status":       analysis.Status,
						"completed_at": analysis.CompletedAt,
					},
				}); err != nil {
					c.Close()
					return
				}
			}

			// Проверяем завершение анализа
			if analysis.Status == "completed" || analysis.Status == "failed" {
				// Send final results and close
				if analysis.Status == "completed" {
					finalResults := getFinalResults(h, analysisID)
					c.WriteJSON(finalResults)
				}

				// Close connection gracefully
				c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Analysis completed"))
				c.Close()
				return
			}

		case <-disconnect:
			// Клиент отключился
			return
		}
	}
}

// Helper function to get final results
func getFinalResults(h *AnalysisHandler, analysisID uuid.UUID) fiber.Map {
	// Get all metrics
	metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return fiber.Map{
			"success": false,
			"error":   "Failed to get metrics",
		}
	}

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

	return fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":              analysisID,
			"status":          "completed",
			"overall_score":   averageScore,
			"category_count":  count,
			"category_scores": categoryScores,
		},
	}
}

// AnalysisRequest представляет запрос на анализ веб-сайта
type AnalysisRequest struct {
	URL string `json:"url" validate:"required,url"`
}
