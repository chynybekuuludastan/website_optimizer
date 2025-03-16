package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

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
	WebSocketHub       *ws.Hub
	activeRequests     sync.Map
}

// NewContentImprovementHandler creates a new content improvement handler
func NewContentImprovementHandler(
	llmService *llm.Service,
	repoFactory *repository.Factory,
	wsHub *ws.Hub,
) *ContentImprovementHandler {
	return &ContentImprovementHandler{
		LLMService:         llmService,
		AnalysisRepo:       repoFactory.AnalysisRepository,
		MetricsRepo:        repoFactory.MetricsRepository,
		ContentImproveRepo: repoFactory.ContentImprovementRepository,
		WebsiteRepo:        repoFactory.WebsiteRepository,
		WebSocketHub:       wsHub,
		activeRequests:     sync.Map{},
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

	// Check if there's already an active generation for this analysis
	if _, exists := h.activeRequests.Load(analysisID.String()); exists {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   "Content improvement generation already in progress",
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

	// Mark this analysis ID as having an active request
	h.activeRequests.Store(analysisID.String(), true)

	// Start content generation in the background with enhanced progress tracking
	go func() {
		defer h.activeRequests.Delete(analysisID.String())
		h.generateContentWithProgressTracking(analysisID, contentRequest, req.ProviderName)
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

// generateContentWithProgressTracking handles content generation with WebSocket progress updates
func (h *ContentImprovementHandler) generateContentWithProgressTracking(
	analysisID uuid.UUID,
	request *llm.ContentRequest,
	providerName string,
) {
	// Track start time for performance metrics
	startTime := time.Now()

	// Send WebSocket notification that generation started
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

	// Create progress callback for live updates
	progressCallback := func(progress float64, message string) {
		// Convert provider progress (0-100) to our progress scale (0-100)
		// We use 40-90 range for LLM generation, with 0-40 for prep and 90-100 for post-processing
		adjustedProgress := 40.0 + (progress * 0.5) // Maps 0-100 to 40-90

		h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:      ws.TypeAnalysisProgress,
			Status:    "processing",
			Progress:  adjustedProgress,
			Category:  ws.CategoryContent,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"stage":    "generating_content",
				"message":  message,
				"progress": adjustedProgress,
			},
		})
	}

	// Initial progress update
	progressCallback(0, "Preparing to generate content")

	// Validate provider
	if providerName == "" {
		// List available providers
		providers := h.LLMService.GetAvailableProviders()
		if len(providers) > 0 {
			providerName = providers[0]
		} else {
			h.sendFailureMessage(analysisID, "No LLM providers available", startTime)
			return
		}
	}

	// Generate content with progress tracking
	response, err := h.LLMService.GenerateContentWithProgress(ctx, request, providerName, progressCallback)
	if err != nil {
		h.sendFailureMessage(analysisID, "Failed to generate content: "+err.Error(), startTime)
		return
	}

	// Send progress update - generating HTML
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeAnalysisProgress,
		Status:    "processing",
		Progress:  90.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":      "generating_html",
			"message":    "Generating HTML from improved content",
			"progress":   90.0,
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
		Progress:  95.0,
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"stage":    "saving_improvements",
			"message":  "Saving content improvements to database",
			"progress": 95.0,
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

// sendFailureMessage sends a standardized failure message via WebSocket
func (h *ContentImprovementHandler) sendFailureMessage(analysisID uuid.UUID, errorMsg string, startTime time.Time) {
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeContentImprovementFailed,
		Status:    "failed",
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"analysis_id": analysisID.String(),
			"error":       errorMsg,
			"message":     "Failed to generate content improvement",
			"duration_ms": time.Since(startTime).Milliseconds(),
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

	// Check generation status
	generationStatus := "not_generated"
	if _, exists := h.activeRequests.Load(analysisID.String()); exists {
		generationStatus = "in_progress"
	} else if len(improvements) > 0 {
		generationStatus = "completed"
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
				"status":       generationStatus,
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
			"status":       generationStatus,
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

// @Summary Cancel content generation
// @Description Cancel an in-progress content improvement generation
// @Tags content-improvements
// @Accept json
// @Produce json
// @Param id path string true "Analysis ID" format="uuid"
// @Success 200 {object} handlers.SuccessResponse "Content generation cancelled"
// @Failure 400 {object} handlers.ErrorResponse "Invalid analysis ID"
// @Failure 404 {object} handlers.ErrorResponse "No active generation found"
// @Security BearerAuth
// @Router /analysis/{id}/content-improvements/cancel [post]
func (h *ContentImprovementHandler) CancelContentGeneration(c *fiber.Ctx) error {
	// Get analysis ID from path
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid analysis ID",
		})
	}

	// Check if there's an active generation
	if _, exists := h.activeRequests.Load(analysisID.String()); !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "No active content generation found for this analysis",
		})
	}

	// Remove from active requests (the actual cancellation is handled by context)
	h.activeRequests.Delete(analysisID.String())

	// Send cancellation notification
	h.WebSocketHub.BroadcastToAnalysis(analysisID, ws.Message{
		Type:      ws.TypeContentImprovementFailed,
		Status:    "cancelled",
		Category:  ws.CategoryContent,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"analysis_id": analysisID.String(),
			"message":     "Content generation was cancelled by user",
		},
	})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Content generation cancelled",
	})
}
