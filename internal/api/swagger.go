package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"

	_ "github.com/chynybekuuludastan/website_optimizer/docs" // This is required for swagger to work
)

// SetupSwagger configures the Swagger routes
func SetupSwagger(app *fiber.App) {
	// Serve swagger documentation
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Redirect root to swagger UI
	app.Get("/swagger", func(c *fiber.Ctx) error {
		return c.Redirect("/swagger/index.html")
	})
}
