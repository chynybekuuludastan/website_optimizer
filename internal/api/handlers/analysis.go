package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
	WebSocketHub       *ws.Hub  // Add this field
	cancelFunctions    sync.Map // For storing cancellation functions
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
	// Получаем ID пользователя из контекста
	userID := c.Locals("userID").(uuid.UUID)

	// Разбираем запрос
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

// runAnalysis performs the analysis with real-time progress reporting via WebSocket
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

	// Create cancellable context that will be used for analysis control
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(a.Config.AnalysisTimeout)*time.Second,
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

	// Parse the website
	websiteData, err := parser.ParseWebsite(url, parser.ParseOptions{
		Timeout: a.Config.AnalysisTimeout,
	})
	if err != nil {
		a.updateAnalysisFailed(analysisID, "Parsing error: "+err.Error())
		return
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

	// Create analyzer manager with progress tracking
	manager := analyzer.NewAnalyzerManager()
	manager.RegisterAllAnalyzers()

	// Register progress callback
	progressChan := make(chan analyzer.ProgressUpdate)
	manager.SetProgressCallback(func(update analyzer.ProgressUpdate) {
		progressChan <- update
	})

	// Start progress reporting goroutine
	go func() {
		for update := range progressChan {
			// Calculate overall progress (15-85% range for analyzers)
			progress := 15.0 + (update.Progress * 70.0 / 100.0)

			a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
				Type:      ws.TypeAnalysisProgress,
				Status:    "analyzing",
				Progress:  progress,
				Category:  ws.AnalysisCategory(update.AnalyzerType),
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"stage":             "analyzing",
					"analyzer":          update.AnalyzerType,
					"analyzer_progress": update.Progress,
					"overall_progress":  progress,
					"message":           update.Message,
					"details":           update.Details,
				},
			})

			// If partial results are available, send them
			if update.PartialResults != nil {
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
	}()

	// Run the analyzers
	results, err := manager.RunAllAnalyzers(ctx, websiteData)
	close(progressChan) // Close the progress channel

	// Check if the analysis was cancelled
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

	// Transaction for saving results
	err = a.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		// Save metrics for each analyzer
		totalMetrics := 0
		for analyzerType, result := range results {
			// Send partial results for each analyzer
			a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
				Type:      ws.TypePartialResults,
				Category:  ws.AnalysisCategory(string(analyzerType)),
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"analyzer": string(analyzerType),
					"results":  result,
					"score":    result["score"],
				},
			})

			// Save to database
			metricData, err := json.Marshal(result)
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

			// Report progress for each saved analyzer (85-95% range)
			progress := 85.0 + (float64(totalMetrics) / float64(len(results)) * 10.0)
			a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
				Type:      ws.TypeAnalysisProgress,
				Status:    "saving",
				Progress:  progress,
				Category:  ws.AnalysisCategory(string(analyzerType)),
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"stage":    "saving_analyzer_results",
					"analyzer": string(analyzerType),
					"progress": progress,
					"message":  "Saving " + string(analyzerType) + " results",
				},
			})
		}

		// Save issues (95-97% range)
		a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeAnalysisProgress,
			Status:    "saving",
			Progress:  95.0,
			Category:  ws.CategoryAll,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"stage":    "saving_issues",
				"message":  "Saving identified issues",
				"progress": 95.0,
			},
		})

		allIssues := manager.GetAllIssues()
		for analyzerType, issues := range allIssues {
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

		// Save recommendations (97-100% range)
		a.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeAnalysisProgress,
			Status:    "saving",
			Progress:  97.0,
			Category:  ws.CategoryAll,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"stage":    "saving_recommendations",
				"message":  "Saving recommendations",
				"progress": 97.0,
			},
		})

		allRecommendations := manager.GetAllRecommendations()
		uniqueRecommendations := make(map[string]struct{})

		for analyzerType, recommendations := range allRecommendations {
			for _, rec := range recommendations {
				// Skip duplicate recommendations
				if _, ok := uniqueRecommendations[rec]; ok {
					continue
				}
				uniqueRecommendations[rec] = struct{}{}

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
		}

		return nil
	})

	if err != nil {
		a.updateAnalysisFailed(analysisID, "Error saving results: "+err.Error())
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

	// Периодическая проверка статуса
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Канал для обработки закрытия соединения
	disconnect := make(chan bool)

	// Обработка входящих сообщений (включая закрытие)
	go func() {
		for {
			messageType, _, err := c.ReadMessage()
			if err != nil || messageType == websocket.CloseMessage {
				disconnect <- true
				break
			}
		}
	}()

	// Основной цикл отправки обновлений
	for {
		select {
		case <-ticker.C:
			// Обновляем данные анализа
			err = h.AnalysisRepo.FindByID(analysisID, &analysis)
			if err != nil {
				c.WriteJSON(fiber.Map{
					"success": false,
					"error":   "Ошибка получения анализа",
				})
				c.Close()
				return
			}

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

			// Проверяем завершение анализа
			if analysis.Status == "completed" || analysis.Status == "failed" {
				// Получаем общий результат, если анализ завершен
				if analysis.Status == "completed" {
					// Получаем все метрики
					metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
					if err != nil {
						c.Close()
						return
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

					// Вычисляем средний балл
					averageScore := 0.0
					if count > 0 {
						averageScore = totalScore / float64(count)
					}

					// Отправляем итоговые результаты
					c.WriteJSON(fiber.Map{
						"success": true,
						"data": fiber.Map{
							"id":              analysis.ID,
							"status":          "completed",
							"completed_at":    analysis.CompletedAt,
							"overall_score":   averageScore,
							"category_count":  count,
							"category_scores": categoryScores,
						},
					})
				}

				// Закрываем соединение после завершения
				c.Close()
				return
			}

		case <-disconnect:
			// Клиент отключился
			return
		}
	}
}

// AnalysisRequest представляет запрос на анализ веб-сайта
type AnalysisRequest struct {
	URL string `json:"url" validate:"required,url"`
}
