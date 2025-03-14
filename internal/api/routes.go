package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"github.com/chynybekuuludastan/website_optimizer/internal/api/handlers"
	"github.com/chynybekuuludastan/website_optimizer/internal/api/middleware"
	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, redisClient, cfg)
	userHandler := handlers.NewUserHandler(db, cfg)
	websiteHandler := handlers.NewWebsiteHandler(db, redisClient, cfg)
	analysisHandler := handlers.NewAnalysisHandler(db, redisClient, cfg)

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
	auth.Post("/refresh", authHandler.RefreshToken)
	auth.Post("/logout", middleware.JWTMiddleware(cfg), authHandler.Logout)
	auth.Get("/me", middleware.JWTMiddleware(cfg), authHandler.GetMe)

	// User routes
	users := api.Group("/users", middleware.JWTMiddleware(cfg))
	users.Get("/", middleware.AdminOnly(), userHandler.ListUsers)
	users.Get("/:id", middleware.Self("id"), userHandler.GetUser)
	users.Put("/:id", middleware.Self("id"), userHandler.UpdateUser)
	users.Delete("/:id", middleware.Self("id"), userHandler.DeleteUser)
	users.Patch("/:id/role", middleware.AdminOnly(), userHandler.UpdateRole)

	// Website routes
	websites := api.Group("/websites", middleware.JWTMiddleware(cfg))
	websites.Post("/", middleware.AnalystOrAdmin(), websiteHandler.CreateWebsite)
	websites.Get("/", middleware.AnalystOrAdmin(), websiteHandler.ListWebsites)
	websites.Get("/:id", middleware.AnalystOrAdmin(), websiteHandler.GetWebsite)
	websites.Delete("/:id", middleware.AnalystOrAdmin(), websiteHandler.DeleteWebsite)

	// Analysis routes
	analysis := api.Group("/analysis")
	analysis.Post("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), analysisHandler.CreateAnalysis)
	analysis.Get("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), analysisHandler.ListAnalyses)
	analysis.Get("/public", analysisHandler.ListPublicAnalyses)

	// Protected analysis routes
	protectedAnalysis := analysis.Group("/:id", middleware.JWTMiddleware(cfg))
	protectedAnalysis.Get("/", analysisHandler.GetAnalysis)
	protectedAnalysis.Delete("/", middleware.AnalystOrAdmin(), analysisHandler.DeleteAnalysis)
	protectedAnalysis.Patch("/public", middleware.AnalystOrAdmin(), analysisHandler.UpdatePublicStatus)

	// Analysis metrics and results
	protectedAnalysis.Get("/metrics", analysisHandler.GetMetrics)
	protectedAnalysis.Get("/metrics/:category", analysisHandler.GetMetricsByCategory)
	protectedAnalysis.Get("/issues", analysisHandler.GetIssues)
	protectedAnalysis.Get("/recommendations", analysisHandler.GetRecommendations)

	// Content improvements
	protectedAnalysis.Get("/content-improvements", analysisHandler.GetContentImprovements)
	protectedAnalysis.Post("/content-improvements", middleware.AnalystOrAdmin(), analysisHandler.GenerateContentImprovements)
	protectedAnalysis.Get("/code-snippets", analysisHandler.GetCodeSnippets)
	protectedAnalysis.Post("/code-snippets", middleware.AnalystOrAdmin(), analysisHandler.GenerateCodeSnippets)

	// WebSocket endpoint for real-time analysis updates
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws/analysis/:id", middleware.JWTMiddleware(cfg), websocket.New(analysisHandler.HandleWebSocket))
}
