package handlers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AnalysisHandler handles website analysis requests
type AnalysisHandler struct {
	DB          *database.DatabaseClient
	RedisClient *database.RedisClient
	Config      *config.Config
	LLMClient   *llm.OpenAIClient
}

// AnalysisRequest represents a request to analyze a website
type AnalysisRequest struct {
	URL string `json:"url" validate:"required,url"`
}

// NewAnalysisHandler creates a new analysis handler
func NewAnalysisHandler(db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) *AnalysisHandler {
	return &AnalysisHandler{
		DB:          db,
		RedisClient: redisClient,
		Config:      cfg,
		LLMClient:   llm.NewOpenAIClient(cfg.OpenAIAPIKey),
	}
}

// CreateAnalysis handles the creation of a new website analysis
func (h *AnalysisHandler) CreateAnalysis(c *fiber.Ctx) error {
	// Get user ID from context
	userID := c.Locals("userID").(uuid.UUID)

	// Parse request
	req := new(AnalysisRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Create or get website
	var website models.Website
	result := h.DB.Where("url = ?", req.URL).First(&website)
	if result.Error != nil {
		// Website not found, create new one
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

	// Create analysis
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

	// Start analysis in background
	go h.runAnalysis(analysis.ID, req.URL)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"analysis_id": analysis.ID,
			"status":      analysis.Status,
		},
	})
}

// runAnalysis performs the actual website analysis
func (h *AnalysisHandler) runAnalysis(analysisID uuid.UUID, url string) {
	// Update analysis status
	if err := h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Update("status", "running").Error; err != nil {
		return
	}

	// Parse website
	websiteData, err := parser.ParseWebsite(url, parser.ParseOptions{Timeout: 30 * time.Second})
	if err != nil {
		h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": time.Now(),
			"metadata":     datatypes.JSON([]byte(`{"error": "` + err.Error() + `"}`)),
		})
		return
	}

	// Update website info
	h.DB.Model(&models.Website{}).Where("url = ?", url).Updates(map[string]interface{}{
		"title":       websiteData.Title,
		"description": websiteData.Description,
	})

	// Run different analyzers
	seoResults := analyzer.AnalyzeSEO(websiteData)
	performanceResults := analyzer.AnalyzePerformance(websiteData)
	securityResults := analyzer.AnalyzeSecurity(websiteData)
	accessibilityResults := analyzer.AnalyzeAccessibility(websiteData)

	// Save metrics to database
	saveMetric(h.DB, analysisID, "seo", "seo_score", seoResults)
	saveMetric(h.DB, analysisID, "performance", "load_time", performanceResults)
	saveMetric(h.DB, analysisID, "security", "security_score", securityResults)
	saveMetric(h.DB, analysisID, "accessibility", "accessibility_score", accessibilityResults)

	// Generate recommendations
	generateRecommendations(h.DB, analysisID, websiteData, seoResults, performanceResults, securityResults, accessibilityResults)

	// Update analysis as completed
	h.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": time.Now(),
	})
}

// saveMetric saves a metric to the database
func saveMetric(db *database.DatabaseClient, analysisID uuid.UUID, category, name string, value interface{}) {
	jsonValue, _ := json.Marshal(value)
	metric := models.AnalysisMetric{
		AnalysisID: analysisID,
		Category:   category,
		Name:       name,
		Value:      datatypes.JSON(jsonValue),
	}
	db.Create(&metric)
}

// generateRecommendations creates recommendations based on analysis results
func generateRecommendations(db *database.DatabaseClient, analysisID uuid.UUID, data *parser.WebsiteData, seoResults, performanceResults, securityResults, accessibilityResults interface{}) {
	// This is a simplified example - in a real application, you would implement more complex logic

	// SEO recommendations
	seoMap, ok := seoResults.(map[string]interface{})
	if ok {
		if seoMap["missing_meta_description"] == true {
			db.Create(&models.Recommendation{
				AnalysisID:  analysisID,
				Category:    "seo",
				Priority:    "high",
				Title:       "Add Meta Description",
				Description: "Your page is missing a meta description tag. Meta descriptions help search engines understand the content of your page.",
				CodeSnippet: `<meta name="description" content="Your description here">`,
			})
		}
	}

	// Performance recommendations
	perfMap, ok := performanceResults.(map[string]interface{})
	if ok {
		if imgSize, exists := perfMap["large_images"].([]map[string]interface{}); exists && len(imgSize) > 0 {
			db.Create(&models.Recommendation{
				AnalysisID:  analysisID,
				Category:    "performance",
				Priority:    "medium",
				Title:       "Optimize Images",
				Description: "Some images on your site are not optimized, which can slow down page loading.",
				CodeSnippet: ``,
			})
		}
	}

	// Here, add more recommendations based on security and accessibility results
}

// GetAnalysis returns the details of a specific analysis
func (h *AnalysisHandler) GetAnalysis(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var analysis models.Analysis
	if err := h.DB.Preload("Website").Preload("User").First(&analysis, analysisID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Check if user can access this analysis
	userID := c.Locals("userID").(uuid.UUID)
	role := c.Locals("role").(string)
	if analysis.UserID != userID && role != "admin" && !analysis.IsPublic {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "You don't have permission to view this analysis",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    analysis,
	})
}

// ListAnalyses lists all analyses for a user
func (h *AnalysisHandler) ListAnalyses(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)
	role := c.Locals("role").(string)

	var analyses []models.Analysis
	query := h.DB.Preload("Website")

	// If not admin, limit to user's own analyses
	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Find(&analyses).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch analyses",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    analyses,
	})
}

// ListPublicAnalyses lists all public analyses
func (h *AnalysisHandler) ListPublicAnalyses(c *fiber.Ctx) error {
	var analyses []models.Analysis
	if err := h.DB.Preload("Website").Where("is_public = ?", true).Find(&analyses).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch public analyses",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    analyses,
	})
}

// DeleteAnalysis deletes an analysis
func (h *AnalysisHandler) DeleteAnalysis(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var analysis models.Analysis
	if err := h.DB.First(&analysis, analysisID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Check if user can delete this analysis
	userID := c.Locals("userID").(uuid.UUID)
	role := c.Locals("role").(string)
	if analysis.UserID != userID && role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "You don't have permission to delete this analysis",
		})
	}

	if err := h.DB.Delete(&analysis).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to delete analysis",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analysis deleted successfully",
	})
}

// UpdatePublicStatus updates the public status of an analysis
func (h *AnalysisHandler) UpdatePublicStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var analysis models.Analysis
	if err := h.DB.First(&analysis, analysisID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	// Check if user can update this analysis
	userID := c.Locals("userID").(uuid.UUID)
	role := c.Locals("role").(string)
	if analysis.UserID != userID && role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "You don't have permission to update this analysis",
		})
	}

	type UpdateRequest struct {
		IsPublic bool `json:"is_public"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if err := h.DB.Model(&analysis).Update("is_public", req.IsPublic).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update analysis public status",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analysis public status updated successfully",
	})
}

// GetMetrics returns all metrics for an analysis
func (h *AnalysisHandler) GetMetrics(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var metrics []models.AnalysisMetric
	if err := h.DB.Where("analysis_id = ?", analysisID).Find(&metrics).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch metrics",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// GetMetricsByCategory returns metrics for an analysis filtered by category
func (h *AnalysisHandler) GetMetricsByCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	category := c.Params("category")

	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var metrics []models.AnalysisMetric
	if err := h.DB.Where("analysis_id = ? AND category = ?", analysisID, category).Find(&metrics).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch metrics",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// GetIssues returns all issues for an analysis
func (h *AnalysisHandler) GetIssues(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var issues []models.Issue
	if err := h.DB.Where("analysis_id = ?", analysisID).Find(&issues).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch issues",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    issues,
	})
}

// GetRecommendations returns all recommendations for an analysis
func (h *AnalysisHandler) GetRecommendations(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var recommendations []models.Recommendation
	if err := h.DB.Where("analysis_id = ?", analysisID).Find(&recommendations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch recommendations",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    recommendations,
	})
}

// GetContentImprovements returns all content improvements for an analysis
func (h *AnalysisHandler) GetContentImprovements(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var improvements []models.ContentImprovement
	if err := h.DB.Where("analysis_id = ?", analysisID).Find(&improvements).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch content improvements",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    improvements,
	})
}

// GenerateContentImprovements generates content improvements using LLM
func (h *AnalysisHandler) GenerateContentImprovements(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var analysis models.Analysis
	if err := h.DB.Preload("Website").First(&analysis, analysisID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis not found",
		})
	}

	type ContentRequest struct {
		Title     string `json:"title"`
		CTAButton string `json:"cta_button"`
		Content   string `json:"content"`
	}

	var req ContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	improvements, err := h.LLMClient.GenerateContentImprovements(
		analysis.Website.URL,
		req.Title,
		req.CTAButton,
		req.Content,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate content improvements: " + err.Error(),
		})
	}

	// Save heading improvement
	if heading, ok := improvements["heading"]; ok {
		h.DB.Create(&models.ContentImprovement{
			AnalysisID:      analysisID,
			ElementType:     "heading",
			OriginalContent: req.Title,
			ImprovedContent: heading,
			LLMModel:        h.LLMClient.Model,
		})
	}

	// Save CTA button improvement
	if cta, ok := improvements["cta_button"]; ok {
		h.DB.Create(&models.ContentImprovement{
			AnalysisID:      analysisID,
			ElementType:     "cta_button",
			OriginalContent: req.CTAButton,
			ImprovedContent: cta,
			LLMModel:        h.LLMClient.Model,
		})
	}

	// Save text content improvement
	if content, ok := improvements["improved_content"]; ok {
		h.DB.Create(&models.ContentImprovement{
			AnalysisID:      analysisID,
			ElementType:     "content",
			OriginalContent: req.Content,
			ImprovedContent: content,
			LLMModel:        h.LLMClient.Model,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    improvements,
	})
}

// GetCodeSnippets returns generated code snippets for an analysis
func (h *AnalysisHandler) GetCodeSnippets(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	var recommendations []models.Recommendation
	if err := h.DB.Where("analysis_id = ? AND code_snippet != ''", analysisID).Find(&recommendations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch code snippets",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    recommendations,
	})
}

// GenerateCodeSnippets generates code snippets for content improvements
func (h *AnalysisHandler) GenerateCodeSnippets(c *fiber.Ctx) error {
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	type CodeRequest struct {
		HTML                string `json:"html"`
		ContentImprovements string `json:"content_improvements"`
	}

	var req CodeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	code, err := h.LLMClient.GenerateCodeSnippet(req.HTML, req.ContentImprovements)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate code snippet: " + err.Error(),
		})
	}

	// Create a recommendation with the generated code
	recommendation := models.Recommendation{
		AnalysisID:  analysisID,
		Category:    "content",
		Priority:    "medium",
		Title:       "Improved HTML Implementation",
		Description: "Generated HTML code with improved content",
		CodeSnippet: code,
	}

	if err := h.DB.Create(&recommendation).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to save code snippet: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"code_snippet": code,
			"id":           recommendation.ID,
		},
	})
}

// HandleWebSocket handles WebSocket connections for real-time analysis updates
func (h *AnalysisHandler) HandleWebSocket(c *websocket.Conn) {
	// Get analysis ID from URL
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		c.Close()
		return
	}

	// Simple implementation - in a real app, you would implement pub/sub
	// to notify when analysis status changes
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var analysis models.Analysis
			if err := h.DB.Select("id, status, completed_at").First(&analysis, analysisID).Error; err != nil {
				c.WriteJSON(fiber.Map{
					"error": "Analysis not found",
				})
				c.Close()
				return
			}

			if err := c.WriteJSON(fiber.Map{
				"id":           analysis.ID,
				"status":       analysis.Status,
				"completed_at": analysis.CompletedAt,
			}); err != nil {
				c.Close()
				return
			}

			// If analysis is completed or failed, close the connection
			if analysis.Status == "completed" || analysis.Status == "failed" {
				c.Close()
				return
			}
		}
	}
}
