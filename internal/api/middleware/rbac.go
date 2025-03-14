package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// RoleMiddleware creates role-based access control middleware
func RoleMiddleware(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get role from context (set by JWTMiddleware)
		role := c.Locals("role")
		if role == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Unauthorized, role information missing",
			})
		}

		// Check if the user's role is in the allowed roles
		userRole := role.(string)
		allowed := false

		for _, r := range allowedRoles {
			if r == userRole {
				allowed = true
				break
			}
		}

		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Forbidden, insufficient permissions",
			})
		}

		return c.Next()
	}
}

// AdminOnly middleware for admin-only routes
func AdminOnly() fiber.Handler {
	return RoleMiddleware("admin")
}

// AnalystOrAdmin middleware for routes accessible by analysts and admins
func AnalystOrAdmin() fiber.Handler {
	return RoleMiddleware("analyst", "admin")
}

// Self middleware ensures the user can only access their own resources
func Self(paramName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user ID from JWT claims
		userID := c.Locals("userID")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Unauthorized, user information missing",
			})
		}

		// Get resource owner ID from params
		resourceOwnerID := c.Params(paramName)
		if resourceOwnerID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Missing resource identifier",
			})
		}

		// Check if user is accessing their own resource
		if userID.(string) != resourceOwnerID {
			// If not, check if user is admin (admins can access everything)
			role := c.Locals("role")
			if role == nil || role.(string) != "admin" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"success": false,
					"error":   "Forbidden, you can only access your own resources",
				})
			}
		}

		return c.Next()
	}
}
