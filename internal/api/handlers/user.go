package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
)

// UserHandler handles user-related requests
type UserHandler struct {
	DB     *database.DatabaseClient
	Config *config.Config
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *database.DatabaseClient, cfg *config.Config) *UserHandler {
	return &UserHandler{
		DB:     db,
		Config: cfg,
	}
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateRoleRequest represents a request to update a user's role
type UpdateRoleRequest struct {
	RoleID uint `json:"role_id" validate:"required"`
}

// ListUsers returns a list of all users
// @Summary List all users
// @Description Get a list of all users in the system
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Users list"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /users [get]
func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := h.DB.Preload("Role").Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch users",
		})
	}

	// Remove sensitive information
	type SafeUser struct {
		ID        uuid.UUID `json:"id"`
		Username  string    `json:"username"`
		Email     string    `json:"email"`
		RoleName  string    `json:"role"`
		CreatedAt string    `json:"created_at"`
	}

	safeUsers := make([]SafeUser, len(users))
	for i, user := range users {
		safeUsers[i] = SafeUser{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			RoleName:  user.Role.Name,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    safeUsers,
	})
}

// GetUser returns information about a specific user
// @Summary Get user details
// @Description Get details of a specific user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{} "User details"
// @Failure 400 {object} map[string]interface{} "Invalid user ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "User not found"
// @Security BearerAuth
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

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
			"updated_at": user.UpdatedAt,
		},
	})
}

// UpdateUser updates a user's information
func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

	req := new(UpdateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "User not found",
		})
	}

	// Update user information
	updates := make(map[string]interface{})

	if req.Username != "" {
		// Check if username is already taken
		var count int64
		h.DB.Model(&models.User{}).Where("username = ? AND id != ?", req.Username, userID).Count(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   "Username already taken",
			})
		}
		updates["username"] = req.Username
	}

	if req.Email != "" {
		// Check if email is already registered
		var count int64
		h.DB.Model(&models.User{}).Where("email = ? AND id != ?", req.Email, userID).Count(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   "Email already registered",
			})
		}
		updates["email"] = req.Email
	}

	if req.Password != "" {
		// Hash new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to hash password",
			})
		}
		updates["password_hash"] = string(hashedPassword)
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to update user",
			})
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User updated successfully",
	})
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "User not found",
		})
	}

	if err := h.DB.Delete(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to delete user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User deleted successfully",
	})
}

// UpdateRole updates a user's role
func (h *UserHandler) UpdateRole(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

	req := new(UpdateRoleRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Check if role exists
	var role models.Role
	if err := h.DB.First(&role, req.RoleID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Role not found",
		})
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "User not found",
		})
	}

	if err := h.DB.Model(&user).Update("role_id", req.RoleID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update user role",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User role updated successfully",
	})
}
