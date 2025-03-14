package api

// This file contains model definitions for Swagger documentation

// AnalysisRequest represents a request to analyze a website
// @Description Analysis request payload
type AnalysisRequest struct {
	URL string `json:"url" example:"https://example.com"` // Website URL to analyze
}

// RegisterRequest represents a request to register a new user
// @Description User registration payload
type RegisterRequest struct {
	Username string `json:"username" example:"johndoe"`          // Username
	Email    string `json:"email" example:"johndoe@example.com"` // Email address
	Password string `json:"password" example:"securePassword"`   // Password
}

// LoginRequest represents a request to log in
// @Description Login payload
type LoginRequest struct {
	Email    string `json:"email" example:"johndoe@example.com"` // Email address
	Password string `json:"password" example:"securePassword"`   // Password
}

// TokenResponse represents a JWT token response
// @Description JWT token response
type TokenResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1..."` // JWT access token
	TokenType   string `json:"token_type" example:"bearer"`                // Token type
	ExpiresIn   int    `json:"expires_in" example:"86400"`                 // Token expiration in seconds
}

// ErrorResponse represents an error response
// @Description Error response
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`       // Success status
	Error   string `json:"error" example:"Error message"` // Error message
}

// SuccessResponse represents a success response
// @Description Success response
type SuccessResponse struct {
	Success bool        `json:"success" example:"true"` // Success status
	Data    interface{} `json:"data"`                   // Response data
}
