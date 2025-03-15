package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"github.com/chynybekuuludastan/website_optimizer/internal/api/handlers"
	"github.com/chynybekuuludastan/website_optimizer/internal/api/middleware"
	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/database"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, db *database.DatabaseClient, redisClient *database.RedisClient, cfg *config.Config) {
	// Initialize repository factory
	repoFactory := repository.NewRepositoryFactory(db.DB)

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

	// Website routes
	websites := api.Group("/websites", middleware.JWTMiddleware(cfg))
	websites.Post("/", middleware.AnalystOrAdmin(), websiteHandler.CreateWebsite)
	websites.Get("/", middleware.AnalystOrAdmin(), websiteHandler.ListWebsites)
	websites.Get("/:id", middleware.AnalystOrAdmin(), websiteHandler.GetWebsite)
	websites.Delete("/:id", middleware.AnalystOrAdmin(), websiteHandler.DeleteWebsite)

	// Analysis routes
	analysis := api.Group("/analysis")
	analysis.Post("/", middleware.JWTMiddleware(cfg), middleware.AnalystOrAdmin(), analysisHandler.CreateAnalysis)

	// Только реализованные методы
	protectedAnalysis := analysis.Group("/:id", middleware.JWTMiddleware(cfg))

	// Новые маршруты для анализаторов (только существующие методы)
	protectedAnalysis.Get("/score", analysisHandler.GetOverallScore)
	protectedAnalysis.Get("/summary/:category", analysisHandler.GetCategorySummary)

	// WebSocket endpoint for real-time analysis updates
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws/analysis/:id", websocket.New(analysisHandler.HandleWebSocket))
}
