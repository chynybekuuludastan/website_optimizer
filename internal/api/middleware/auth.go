package middleware

import (
	"strings"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// JWTClaims represents JWT claims structure
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// JWTMiddleware creates JWT auth middleware
func JWTMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Authorization header is required",
			})
		}

		// Parse the token from the Authorization header
		tokenString := ""
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid authorization format",
			})
		}

		// Parse and validate the token
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid or expired token",
			})
		}

		// Add claims to context for later use
		c.Locals("userID", claims.UserID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// GenerateJWT creates a new JWT token
func GenerateJWT(user *models.User, role string, secret string, expiration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID: user.ID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
