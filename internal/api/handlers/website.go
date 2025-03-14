package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
)

// WebsiteHandler handles website-related requests
type WebsiteHandler struct {
	DB          *database.DatabaseClient
	RedisClient *database.RedisClient
	Config      *config.Config
}

// NewWebsiteHandler creates a new website handler
func NewWebsiteHandler(db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) *WebsiteHandler {
	return &WebsiteHandler{
		DB:          db,
		RedisClient: redisClient,
		Config:      cfg,
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
	var website models.Website
	result := h.DB.Where("url = ?", req.URL).First(&website)
	if result.Error == nil {
		// Return existing website
		return c.JSON(fiber.Map{
			"success": true,
			"data":    website,
		})
	}

	// Create new website
	website = models.Website{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := h.DB.Create(&website).Error; err != nil {
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
	var websites []models.Website
	if err := h.DB.Find(&websites).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch websites: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    websites,
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

	var website models.Website
	if err := h.DB.First(&website, websiteID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Website not found",
		})
	}

	// Get analyses count for this website
	var analysesCount int64
	h.DB.Model(&models.Analysis{}).Where("website_id = ?", websiteID).Count(&analysesCount)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":             website.ID,
			"url":            website.URL,
			"title":          website.Title,
			"description":    website.Description,
			"created_at":     website.CreatedAt,
			"updated_at":     website.UpdatedAt,
			"analyses_count": analysesCount,
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
	if err := h.DB.First(&website, websiteID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Website not found",
		})
	}

	// Check if any analyses reference this website
	var count int64
	h.DB.Model(&models.Analysis{}).Where("website_id = ?", websiteID).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Cannot delete website with existing analyses",
		})
	}

	if err := h.DB.Delete(&website).Error; err != nil {
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
