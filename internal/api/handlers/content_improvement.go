package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/chynybekuuludastan/website_optimizer/internal/api/websocket"
	ws "github.com/chynybekuuludastan/website_optimizer/internal/api/websocket"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
)

// ContentImprovementHandler handles requests for content improvements
type ContentImprovementHandler struct {
	LLMService         *llm.Service
	AnalysisRepo       repository.AnalysisRepository
	MetricsRepo        repository.MetricsRepository
	ContentImproveRepo repository.ContentImprovementRepository
	WebsiteRepo        repository.WebsiteRepository
	WebSocketHub       *websocket.Hub
}

// NewContentImprovementHandler creates a new content improvement handler
func NewContentImprovementHandler(
	llmService *llm.Service,
	repoFactory *repository.Factory,
	wsHub *websocket.Hub,
) *ContentImprovementHandler {
	return &ContentImprovementHandler{
		LLMService:         llmService,
		AnalysisRepo:       repoFactory.AnalysisRepository,
		MetricsRepo:        repoFactory.MetricsRepository,
		ContentImproveRepo: repoFactory.ContentImprovementRepository,
		WebsiteRepo:        repoFactory.WebsiteRepository,
		WebSocketHub:       wsHub,
	}
}

// ContentImprovementRequest represents a request for content improvement
type ContentImprovementRequest struct {
	TargetAudience string `json:"target_audience"`
	Language       string `json:"language"`
	ProviderName   string `json:"provider"`
}

type SuccessResponse struct {
	Success bool        `json:"success" example:"true"`
	Message string      `json:"message,omitempty" example:"Operation completed successfully"`
	Data    interface{} `json:"data,omitempty" swaggertype:"object"`
}

// ContentImprovementResponse represents the formatted content improvements response
type ContentImprovementResponse struct {
	Success bool                   `json:"success" example:"true"`
	Data    ContentImprovementData `json:"data"`
}

// ContentImprovementData represents the data part of the content improvements response
type ContentImprovementData struct {
	Improvements map[string]string `json:"improvements" example:"{'heading':'Improved Heading','cta':'Click Now','content':'Better content text'}"`
	Status       string            `json:"status" example:"completed"`
	Model        string            `json:"model,omitempty" example:"gemini"`
	CreatedAt    string            `json:"created_at,omitempty" example:"2025-03-16T12:00:00Z"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Error   string `json:"error" example:"Something went wrong"`
}

// @Summary Request new content improvement
// @Description Generate new content improvements using LLM for a specific analysis
// @Tags content-improvements
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID" format="uuid"
// @Param request body handlers.ContentImprovementRequest true "Content improvement request parameters"
// @Success 202 {object} handlers.SuccessResponse "Content improvement generation initiated"
// @Failure 400 {object} handlers.ErrorResponse "Invalid request"
// @Failure 401 {object} handlers.ErrorResponse "Unauthorized"
// @Failure 403 {object} handlers.ErrorResponse "Forbidden"
// @Failure 404 {object} handlers.ErrorResponse "Analysis not found"
// @Failure 500 {object} handlers.ErrorResponse "Server error"
// @Security BearerAuth
// @Router /analysis/{id}/content-improvements [post]
func (h *ContentImprovementHandler) RequestContentImprovement(c *fiber.Ctx) error {
	// Get analysis ID from path
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Parse request body
	req := new(ContentImprovementRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
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

	// Check if analysis is completed
	if analysis.Status != "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Analysis is not completed yet",
			"status":  analysis.Status,
		})
	}

	// Get website data
	var website models.Website
	if err := h.WebsiteRepo.FindByID(analysis.WebsiteID, &website); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch website data",
		})
	}

	// Get metrics data for analysis results
	metrics, err := h.MetricsRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch analysis metrics",
		})
	}

	// Extract text content from website or analysis metadata
	title := website.Title
	content := ""
	ctaText := "Learn More" // Default CTA text

	// Extract more details from analysis metadata if available
	if analysis.Metadata != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(analysis.Metadata, &metadata); err == nil {
			if t, ok := metadata["page_title"].(string); ok && t != "" {
				title = t
			}
			if c, ok := metadata["page_content"].(string); ok && c != "" {
				content = c
			}
			if cta, ok := metadata["cta_text"].(string); ok && cta != "" {
				ctaText = cta
			}
		}
	}

	// Use website description if content is still empty
	if content == "" {
		content = website.Description
	}

	// Check for minimum required content
	if title == "" {
		title = "Untitled Page"
	}
	if content == "" {
		content = "No content available for analysis."
	}

	// Extract analysis results
	analysisResults := llm.ExtractAnalysisResults(metrics)

	// Create a ContentRequest
	contentRequest := &llm.ContentRequest{
		URL:             website.URL,
		Title:           title,
		CTAText:         ctaText,
		Content:         content,
		AnalysisResults: analysisResults,
		Language:        req.Language,
		TargetAudience:  req.TargetAudience,
	}

	// Start content generation in the background
	go func() {
		h.generateAndSaveContentImprovements(analysisID, contentRequest, req.ProviderName)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "Content improvement generation started",
		"data": fiber.Map{
			"analysis_id": analysisID,
			"status":      "processing",
		},
	})
}

func (h *ContentImprovementHandler) generateAndSaveContentImprovements(
	analysisID uuid.UUID,
	request *llm.ContentRequest,
	providerName string,
) {
	// Track start time for performance metrics
	startTime := time.Now()

	// Send WebSocket notification that generation started with enhanced message format
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeContentImprovementStarted,
		Status:    "processing",
		Progress:  0.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"analysis_id":     analysisID.String(),
			"provider":        providerName,
			"target_audience": request.TargetAudience,
			"language":        request.Language,
			"message":         "Content improvement generation started",
		},
	})

	// Set timeout for generation
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Send progress update - analysis results obtained
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "processing",
		Progress:  20.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "preparing_content",
			"message":  "Processing analysis results for content improvement",
			"progress": 20.0,
		},
	})

	// Send progress update - sending to LLM
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "processing",
		Progress:  40.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "generating_content",
			"message":  "Sending request to language model",
			"progress": 40.0,
			"provider": providerName,
		},
	})

	// Generate content
	response, err := h.LLMService.GenerateContent(ctx, request, providerName)
	if err != nil {
		// Log error and send failure notification
		h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeContentImprovementFailed,
			Status:    "failed",
			Category:  ws.CategoryContent,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"error":       err.Error(),
				"message":     "Failed to generate content improvement",
				"duration_ms": time.Since(startTime).Milliseconds(),
			},
		})
		return
	}

	// Send progress update - generating HTML
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "processing",
		Progress:  70.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":      "generating_html",
			"message":    "Generating HTML from improved content",
			"progress":   70.0,
			"provider":   providerName,
			"model_used": response.ProviderUsed,
		},
	})

	// Generate HTML
	html, err := h.LLMService.GenerateHTML(ctx, request, response, providerName)
	if err != nil {
		// Continue without HTML, send warning
		h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeWarning,
			Category:  ws.CategoryContent,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"warning":     "HTML generation failed but content was created",
				"error":       err.Error(),
				"stage":       "html_generation",
			},
		})
	} else {
		response.HTML = html
	}

	// Send progress update - saving to database
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "processing",
		Progress:  90.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "saving_improvements",
			"message":  "Saving content improvements to database",
			"progress": 90.0,
			"has_html": html != "",
		},
	})

	// Prepare database records
	improvements := []models.ContentImprovement{
		{
			AnalysisID:      analysisID,
			ElementType:     "heading",
			OriginalContent: request.Title,
			ImprovedContent: response.Title,
			LLMModel:        response.ProviderUsed,
		},
		{
			AnalysisID:      analysisID,
			ElementType:     "cta",
			OriginalContent: request.CTAText,
			ImprovedContent: response.CTAText,
			LLMModel:        response.ProviderUsed,
		},
		{
			AnalysisID:      analysisID,
			ElementType:     "content",
			OriginalContent: request.Content,
			ImprovedContent: response.Content,
			LLMModel:        response.ProviderUsed,
		},
	}

	// Add HTML as a separate improvement if available
	if response.HTML != "" {
		improvements = append(improvements, models.ContentImprovement{
			AnalysisID:      analysisID,
			ElementType:     "html",
			OriginalContent: "",
			ImprovedContent: response.HTML,
			LLMModel:        response.ProviderUsed,
		})
	}

	// Save all improvements
	err = h.ContentImproveRepo.CreateBatch(improvements)
	if err != nil {
		// Send notification of partial success
		h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeContentImprovementFailed,
			Status:    "completed_with_errors",
			Category:  ws.CategoryContent,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"error":       "Generated content couldn't be saved: " + err.Error(),
				"message":     "Content was generated but couldn't be saved",
				"content": map[string]interface{}{
					"heading": response.Title,
					"cta":     response.CTAText,
					"content": response.Content[0:100] + "...", // Include preview
				},
				"duration_ms": time.Since(startTime).Milliseconds(),
			},
		})
		return
	}

	// Send WebSocket notification that generation is complete with detailed info
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeContentImprovementCompleted,
		Status:    "completed",
		Progress:  100.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"analysis_id":   analysisID.String(),
			"provider_used": response.ProviderUsed,
			"cached":        response.CachedResult,
			"has_html":      response.HTML != "",
			"duration_ms":   time.Since(startTime).Milliseconds(),
			"improvements": map[string]string{
				"heading": response.Title,
				"cta":     response.CTAText,
				"content": response.Content,
			},
		},
	})
}

// @Summary Get content improvements for an analysis
// @Description Retrieve all content improvements generated for a specific analysis
// @Tags content-improvements
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID" format="uuid"
// @Success 200 {object} handlers.SuccessResponse "Content improvements retrieved successfully"
// @Failure 400 {object} handlers.ErrorResponse "Invalid analysis ID"
// @Failure 401 {object} handlers.ErrorResponse "Unauthorized"
// @Failure 404 {object} handlers.ErrorResponse "Analysis not found"
// @Failure 500 {object} handlers.ErrorResponse "Failed to fetch content improvements"
// @Security BearerAuth
// @Router /analysis/{id}/content-improvements [get]
func (h *ContentImprovementHandler) GetContentImprovements(c *fiber.Ctx) error {
	// Get analysis ID from path
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Get content improvements for this analysis
	improvements, err := h.ContentImproveRepo.FindByAnalysisID(analysisID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch content improvements",
		})
	}

	// If no improvements found, check if analysis exists
	if len(improvements) == 0 {
		var analysis models.Analysis
		if err := h.AnalysisRepo.FindByID(analysisID, &analysis); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Analysis not found",
			})
		}

		// Analysis exists but no improvements yet
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"improvements": []interface{}{},
				"status":       "not_generated",
			},
		})
	}

	// Format response by element type
	response := map[string]string{}
	for _, improvement := range improvements {
		response[improvement.ElementType] = improvement.ImprovedContent
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"improvements": response,
			"status":       "completed",
			"model":        improvements[0].LLMModel,
			"created_at":   improvements[0].CreatedAt,
		},
	})
}

// @Summary Get HTML content directly
// @Description Retrieve generated HTML content for a specific analysis
// @Tags content-improvements
// @Accept json
// @Produce html
// @Param id path string true "Analysis ID" format="uuid"
// @Success 200 {string} string "HTML content"
// @Failure 400 {object} handlers.ErrorResponse "Invalid analysis ID"
// @Failure 401 {object} handlers.ErrorResponse "Unauthorized"
// @Failure 404 {object} handlers.ErrorResponse "HTML content not found"
// @Failure 500 {object} handlers.ErrorResponse "Server error"
// @Security BearerAuth
// @Router /analysis/{id}/content-html [get]
func (h *ContentImprovementHandler) GetContentHTML(c *fiber.Ctx) error {
	// Get analysis ID from path
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Find HTML improvement
	improvements, err := h.ContentImproveRepo.FindByElementType(analysisID, "html")
	if err != nil || len(improvements) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "HTML content not found",
		})
	}

	// Return HTML content directly
	return c.Status(fiber.StatusOK).
		Type("html").
		SendString(improvements[0].ImprovedContent)
}
