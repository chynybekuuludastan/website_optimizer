package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"golang.org/x/time/rate"

	"github.com/chynybekuuludastan/website_optimizer/internal/api/handlers"
	"github.com/chynybekuuludastan/website_optimizer/internal/api/middleware"
	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm/providers"
)

// @title Website Optimizer API
// @version 1.0
// @description API for website analysis and content improvement using LLM
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url https://website-optimizer.com/support
// @contact.email support@website-optimizer.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token

func SetupRoutes(app *fiber.App, db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) {
	// Initialize repository factory
	repoFactory := repository.NewRepositoryFactory(db.DB, redisClient.Client)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(repoFactory.UserRepository, redisClient, cfg)
	userHandler := handlers.NewUserHandler(repoFactory.UserRepository, cfg)
	websiteHandler := handlers.NewWebsiteHandler(
		repoFactory.WebsiteRepository,
		repoFactory.AnalysisRepository,
		redisClient,
		cfg,
	)
	analysisHandler := handlers.NewAnalysisHandler(repoFactory, redisClient, cfg)

	// Serve static files
	app.Static("/static", "./static")

	// API group
	api := app.Group("/api")

	// Health check route
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// Auth routes
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", middleware.JWTMiddleware(cfg), authHandler.RefreshToken)
	auth.Post("/logout", middleware.JWTMiddleware(cfg), authHandler.Logout)
	auth.Get("/me", middleware.JWTMiddleware(cfg), authHandler.GetMe)

	// User routes
	users := api.Group("/users", middleware.JWTMiddleware(cfg))
	users.Get("/", middleware.AdminOnly(), userHandler.ListUsers)
	users.Get("/:id", middleware.Self("id"), userHandler.GetUser)
	users.Put("/:id", middleware.Self("id"), userHandler.UpdateUser)
	users.Delete("/:id", middleware.Self("id"), userHandler.DeleteUser)
	users.Patch("/:id/role", middleware.AdminOnly(), userHandler.UpdateRole)

	// internal/api/routes.go
	websites := api.Group("/websites", middleware.JWTMiddleware(cfg))
	websites.Post("/", middleware.AnalystOrAdmin(), websiteHandler.CreateWebsite)
	websites.Get("/", middleware.AnalystOrAdmin(), websiteHandler.ListWebsites)
	websites.Get("/popular", middleware.AnalystOrAdmin(), websiteHandler.GetPopularWebsites)                     // New endpoint
	websites.Get("/domains/:domain/statistics", middleware.AnalystOrAdmin(), websiteHandler.GetDomainStatistics) // New endpoint
	websites.Get("/:id", middleware.AnalystOrAdmin(), websiteHandler.GetWebsite)
	websites.Delete("/:id", middleware.AnalystOrAdmin(), websiteHandler.DeleteWebsite)

	// Analysis routes
	// Setup additional analysis routes
	analysis := api.Group("/analysis")
	analysis.Post("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), analysisHandler.CreateAnalysis)
	analysis.Get("/latest", middleware.JWTMiddleware(cfg), analysisHandler.GetLatestAnalyses)
	analysis.Get("/statistics", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), analysisHandler.GetAnalyticsStatistics)

	// Protected analysis routes with appropriate authorization
	protectedAnalysis := analysis.Group("/:id", middleware.JWTMiddleware(cfg))
	protectedAnalysis.Get("/metrics", middleware.AnalystOrAdmin(), analysisHandler.GetAnalysisMetrics)
	protectedAnalysis.Get("/metrics/:category", middleware.AnalystOrAdmin(), analysisHandler.GetAnalysisMetricsByCategory)
	protectedAnalysis.Get("/issues", middleware.AnalystOrAdmin(), analysisHandler.GetAnalysisIssues)
	protectedAnalysis.Get("/score", analysisHandler.GetOverallScore)
	protectedAnalysis.Get("/summary/:category", analysisHandler.GetCategorySummary)
	protectedAnalysis.Patch("/metadata", middleware.AnalystOrAdmin(), analysisHandler.UpdateMetadata)

	// Setup LLM related routes
	setupLLMRoutes(api, repoFactory, redisClient, cfg)

	// Set up Swagger documentation endpoint
	app.Get("/swagger/*", swagger.HandlerDefault)
}

// Setup LLM related routes
func setupLLMRoutes(apiGroup fiber.Router, repoFactory *repository.Factory, redisClient *database.RedisClient, cfg *config.Config) {
	// Initialize LLM service with the internal redis.Client
	llmService := llm.NewService(llm.ServiceOptions{
		DefaultProvider: "gemini",
		RedisClient:     redisClient.Client,
		RateLimit:       rate.Limit(5), // 5 requests per second
		RateBurst:       2,
		CacheTTL:        24 * time.Hour,
		MaxRetries:      3,
		RetryDelay:      time.Second,
		DefaultTimeout:  2 * time.Minute, // Set a reasonable timeout
	})

	// Create Gemini provider if config exists
	geminiProvider, err := providers.NewGeminiProvider(cfg.GeminiAPIKey, "gemini-1.5-flash", nil)
	if err == nil {
		llmService.RegisterProvider(geminiProvider)
	}

	// Initialize content improvement handler
	contentHandler := handlers.NewContentImprovementHandler(llmService, repoFactory, redisClient)

	// Set up routes for content improvements
	contentRoutes := apiGroup.Group("/analysis/:id/content-improvements")
	contentRoutes.Get("/", middleware.JWTMiddleware(cfg), contentHandler.GetContentImprovements)
	contentRoutes.Post("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), contentHandler.RequestContentImprovement)
	contentRoutes.Post("/cancel", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), contentHandler.CancelContentGeneration)
	// Set up code snippet routes
	codeSnippetRoutes := apiGroup.Group("/analysis/:id/code-snippets")
	codeSnippetRoutes.Get("/", middleware.JWTMiddleware(cfg), contentHandler.GetCodeSnippets)
	codeSnippetRoutes.Post("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), contentHandler.GenerateCodeSnippets)

	// HTML content route
	apiGroup.Get("/analysis/:id/content-html", middleware.JWTMiddleware(cfg), contentHandler.GetContentHTML)

	// LLM providers info route - useful for the frontend
	apiGroup.Get("/llm/providers", middleware.JWTMiddleware(cfg), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"providers": llmService.GetAvailableProviders(),
				"default":   cfg.DefaultLLMProvider,
			},
		})
	})
}
