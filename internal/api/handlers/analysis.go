package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AnalysisHandler обрабатывает запросы по анализу веб-сайтов
type AnalysisHandler struct {
	DB          *database.DatabaseClient
	RedisClient *database.RedisClient
	Config      *config.Config
}

// NewAnalysisHandler создает новый обработчик анализа
func NewAnalysisHandler(db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) *AnalysisHandler {
	return &AnalysisHandler{
		DB:          db,
		RedisClient: redisClient,
		Config:      cfg,
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
	var website models.Website
	result := h.DB.Where("url = ?", req.URL).First(&website)
	if result.Error != nil {
		// Веб-сайт не найден, создаем новый
		website = models.Website{
			URL: req.URL,
		}
		if err := h.DB.Create(&website).Error; err != nil {
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

	if err := h.DB.Create(&analysis).Error; err != nil {
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

// runAnalysis выполняет фактический анализ веб-сайта
func (h *AnalysisHandler) runAnalysis(analysisID uuid.UUID, url string) {
	// Обновляем статус анализа
	if err := h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Update("status", "running").Error; err != nil {
		return
	}

	// Парсим веб-сайт
	websiteData, err := parser.ParseWebsite(url, parser.ParseOptions{Timeout: h.Config.AnalysisTimeout})
	if err != nil {
		h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": time.Now(),
			"metadata":     datatypes.JSON([]byte(`{"error": "` + err.Error() + `"}`)),
		})
		return
	}

	// Обновляем информацию о веб-сайте
	h.DB.Model(&models.Website{}).Where("url = ?", url).Updates(map[string]interface{}{
		"title":       websiteData.Title,
		"description": websiteData.Description,
	})

	// Создаем менеджер анализаторов
	manager := analyzer.NewAnalyzerManager()
	manager.RegisterAllAnalyzers()

	// Запускаем все анализаторы
	results, err := manager.RunAllAnalyzers(websiteData)
	if err != nil {
		h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": time.Now(),
			"metadata":     datatypes.JSON([]byte(`{"error": "` + err.Error() + `"}`)),
		})
		return
	}

	// Сохраняем метрики в базу данных
	for analyzerType, result := range results {
		metricData, _ := json.Marshal(result)
		metric := models.AnalysisMetric{
			AnalysisID: analysisID,
			Category:   string(analyzerType),
			Name:       string(analyzerType) + "_score",
			Value:      datatypes.JSON(metricData),
		}
		h.DB.Create(&metric)
	}

	// Сохраняем проблемы
	allIssues := manager.GetAllIssues()
	for analyzerType, issues := range allIssues {
		for _, issue := range issues {
			severity := issue["severity"].(string)
			description := issue["description"].(string)

			// Создаем запись о проблеме
			issueRecord := models.Issue{
				AnalysisID:  analysisID,
				Category:    string(analyzerType),
				Severity:    severity,
				Title:       description,
				Description: description,
			}

			// Если есть дополнительные данные, сохраняем их в Location
			if location, ok := issue["url"].(string); ok {
				issueRecord.Location = location
			} else if count, ok := issue["count"].(int); ok {
				issueRecord.Location = fmt.Sprintf("Count: %d", count)
			}

			h.DB.Create(&issueRecord)
		}
	}

	// Сохраняем рекомендации
	allRecommendations := manager.GetAllRecommendations()
	for analyzerType, recommendations := range allRecommendations {
		for _, rec := range recommendations {
			priority := "medium" // Значение по умолчанию

			// Определение приоритета на основе типа анализатора
			switch analyzerType {
			case analyzer.SEOType, analyzer.SecurityType:
				priority = "high"
			case analyzer.PerformanceType, analyzer.AccessibilityType:
				priority = "medium"
			case analyzer.StructureType, analyzer.MobileType, analyzer.ContentType:
				priority = "low"
			}

			// Создаем запись рекомендации
			recommendation := models.Recommendation{
				AnalysisID:  analysisID,
				Category:    string(analyzerType),
				Priority:    priority,
				Title:       rec,
				Description: rec,
			}

			h.DB.Create(&recommendation)
		}
	}

	// Обновляем анализ как завершенный
	h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": time.Now(),
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

	var metrics []models.AnalysisMetric
	if err := h.DB.Where("analysis_id = ?", analysisID).Find(&metrics).Error; err != nil {
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
	if err := h.DB.Select("id, status, metadata").First(&analysis, analysisID).Error; err != nil {
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

	// Получаем метрики для категории
	var metric models.AnalysisMetric
	if err := h.DB.Where("analysis_id = ? AND category = ?", analysisID, category).First(&metric).Error; err != nil {
		// Instead of 404, return a more informative response
		if err == gorm.ErrRecordNotFound {
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

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Не удалось получить метрики: " + err.Error(),
		})
	}

	// Получаем проблемы и рекомендации
	var issues []models.Issue
	h.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Find(&issues)

	var recommendations []models.Recommendation
	h.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Find(&recommendations)

	// Парсим результаты метрик
	var result map[string]interface{}
	if err := json.Unmarshal(metric.Value, &result); err != nil {
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
	if err := h.DB.Select("id, status, completed_at").First(&analysis, analysisID).Error; err != nil {
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
			if err := h.DB.Select("id, status, completed_at").First(&analysis, analysisID).Error; err != nil {
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
					// Вычисляем общий балл
					var metrics []models.AnalysisMetric
					h.DB.Where("analysis_id = ?", analysisID).Find(&metrics)

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
