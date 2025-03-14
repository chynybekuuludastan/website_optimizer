package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/chynybekuuludastan/website_optimizer/internal/api/middleware"
	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	DB          *database.DatabaseClient
	RedisClient *database.RedisClient
	Config      *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		DB:          db,
		RedisClient: redisClient,
		Config:      cfg,
	}
}

// RegisterRequest represents a request to register a new user
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents a request to log in
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// TokenResponse represents a JWT token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// @Summary Register a new user
// @Description Register a new user in the system
// @Tags auth
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "User Registration"
// @Success 201 {object} map[string]interface{} "User created successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 409 {object} map[string]interface{} "User already exists"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	req := new(RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Check if user already exists
	var count int64
	h.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   "Email already registered",
		})
	}

	h.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   "Username already taken",
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to hash password",
		})
	}

	// Get analyst role (default for new users)
	var role models.Role
	if err := h.DB.Where("name = ?", "analyst").First(&role).Error; err != nil {
		// If analyst role doesn't exist, create default roles
		seedDefaultRoles(h.DB)
		h.DB.Where("name = ?", "analyst").First(&role)
	}

	// Create user
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		RoleID:       role.ID,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// @Summary User login
// @Description Authenticate a user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login Credentials"
// @Success 200 {object} map[string]interface{} "Login successful"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Invalid credentials"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	req := new(LoginRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Find user by email
	var user models.User
	if err := h.DB.Preload("Role").Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid credentials",
		})
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid credentials",
		})
	}

	// Generate JWT token
	token, err := middleware.GenerateJWT(&user, user.Role.Name, h.Config.JWTSecret, h.Config.JWTExpiration)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate token",
		})
	}

	// Store token in Redis for blacklisting on logout
	tokenKey := "token:" + token
	h.RedisClient.Set(tokenKey, true, h.Config.JWTExpiration)

	return c.JSON(fiber.Map{
		"success": true,
		"data": TokenResponse{
			AccessToken: token,
			TokenType:   "bearer",
			ExpiresIn:   int(h.Config.JWTExpiration.Seconds()),
		},
	})
}

// RefreshToken refreshes a JWT token
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// Get user ID from JWT middleware
	userID := c.Locals("userID").(uuid.UUID)

	// Find user
	var user models.User
	if err := h.DB.Preload("Role").First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user",
		})
	}

	// Generate new token
	token, err := middleware.GenerateJWT(&user, user.Role.Name, h.Config.JWTSecret, h.Config.JWTExpiration)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate token",
		})
	}

	// Store token in Redis
	tokenKey := "token:" + token
	h.RedisClient.Set(tokenKey, true, h.Config.JWTExpiration)

	return c.JSON(fiber.Map{
		"success": true,
		"data": TokenResponse{
			AccessToken: token,
			TokenType:   "bearer",
			ExpiresIn:   int(h.Config.JWTExpiration.Seconds()),
		},
	})
}

// Logout invalidates a JWT token
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Get token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Missing token",
		})
	}

	// Extract token
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid token format",
		})
	}

	// Add token to blacklist in Redis
	tokenKey := "token:" + token
	h.RedisClient.Set(tokenKey, false, h.Config.JWTExpiration)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logged out successfully",
	})
}

// GetMe returns information about the current user
func (h *AuthHandler) GetMe(c *fiber.Ctx) error {
	// Get user ID from JWT middleware
	userID := c.Locals("userID").(uuid.UUID)

	// Find user
	var user models.User
	if err := h.DB.Preload("Role").First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"role":       user.Role.Name,
			"created_at": user.CreatedAt,
		},
	})
}

// seedDefaultRoles seeds default roles if they don't exist
func seedDefaultRoles(db *database.DatabaseClient) error {
	var count int64
	db.Model(&models.Role{}).Count(&count)
	if count > 0 {
		return nil
	}

	roles := []models.Role{
		{Name: "admin", Description: "Administrator with full access"},
		{Name: "analyst", Description: "User who can analyze websites"},
		{Name: "guest", Description: "Limited access user"},
	}

	return db.Create(&roles).Error
}
