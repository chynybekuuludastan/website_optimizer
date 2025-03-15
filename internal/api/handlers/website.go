// internal/api/handlers/website.go
package handlers

import (
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
)

// WebsiteHandler handles website-related requests
type WebsiteHandler struct {
	WebsiteRepo  repository.WebsiteRepository
	AnalysisRepo repository.AnalysisRepository
	RedisClient  *database.RedisClient
	Config       *config.Config
}

// NewWebsiteHandler creates a new website handler
func NewWebsiteHandler(websiteRepo repository.WebsiteRepository,
	analysisRepo repository.AnalysisRepository,
	redisClient *database.RedisClient,
	cfg *config.Config) *WebsiteHandler {
	return &WebsiteHandler{
		WebsiteRepo:  websiteRepo,
		AnalysisRepo: analysisRepo,
		RedisClient:  redisClient,
		Config:       cfg,
	}
}

// CreateWebsiteRequest represents a request to create a website
type CreateWebsiteRequest struct {
	URL         string `json:"url" validate:"required,url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// @Summary Create a new website
// @Description Create a new website record
// @Tags websites
// @Accept json
// @Produce json
// @Param website body CreateWebsiteRequest true "Website Information"
// @Success 201 {object} map[string]interface{} "Website created"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /websites [post]
func (h *WebsiteHandler) CreateWebsite(c *fiber.Ctx) error {
	req := new(CreateWebsiteRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Check if website already exists
	exists, err := h.WebsiteRepo.ExistsByURL(req.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error: " + err.Error(),
		})
	}

	if exists {
		// Return existing website
		website, err := h.WebsiteRepo.FindByURL(req.URL)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to fetch website: " + err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data":    website,
		})
	}

	// Create new website
	website := models.Website{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := h.WebsiteRepo.Create(&website); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create website: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    website,
	})
}

// @Summary List all websites
// @Description Get a list of all websites in the system
// @Tags websites
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Websites list"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /websites [get]
func (h *WebsiteHandler) ListWebsites(c *fiber.Ctx) error {
	// Get page and page size from query parameters or use defaults
	page := 1
	pageSize := 10

	if c.Query("page") != "" {
		if pageInt, err := strconv.Atoi(c.Query("page")); err == nil && pageInt > 0 {
			page = pageInt
		}
	}

	if c.Query("per_page") != "" {
		if sizeInt, err := strconv.Atoi(c.Query("per_page")); err == nil && sizeInt > 0 {
			pageSize = sizeInt
		}
	}

	// Check if we have a search query
	searchQuery := c.Query("search")

	var websites []*models.Website
	var total int64
	var err error

	if searchQuery != "" {
		// Search websites
		websites, total, err = h.WebsiteRepo.Search(searchQuery, page, pageSize)
	} else {
		// Get all websites with pagination
		websites, total, err = h.WebsiteRepo.FindAll(page, pageSize)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch websites: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    websites,
		"meta": fiber.Map{
			"total":       total,
			"page":        page,
			"per_page":    pageSize,
			"total_pages": int(math.Ceil(float64(total) / float64(pageSize))),
		},
	})
}

// GetWebsite returns information about a specific website
func (h *WebsiteHandler) GetWebsite(c *fiber.Ctx) error {
	id := c.Params("id")
	websiteID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid website ID",
		})
	}

	// Get website with its analyses
	website, analyses, err := h.WebsiteRepo.FindWithAnalyses(websiteID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Website not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":             website.ID,
			"url":            website.URL,
			"title":          website.Title,
			"description":    website.Description,
			"created_at":     website.CreatedAt,
			"updated_at":     website.UpdatedAt,
			"analyses_count": len(analyses),
			"analyses":       analyses,
		},
	})
}

// DeleteWebsite deletes a website
func (h *WebsiteHandler) DeleteWebsite(c *fiber.Ctx) error {
	id := c.Params("id")
	websiteID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid website ID",
		})
	}

	var website models.Website
	err = h.WebsiteRepo.FindByID(websiteID, &website)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Website not found",
		})
	}

	// Check if any analyses reference this website
	_, count, err := h.AnalysisRepo.FindByWebsiteID(websiteID, 1, 1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to check references: " + err.Error(),
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Cannot delete website with existing analyses",
		})
	}

	if err := h.WebsiteRepo.Delete(&website); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to delete website: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Website deleted successfully",
	})
}
