package seed

import (
	"log"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedDefaultRoles seeds default roles if they don't exist
func SeedDefaultRoles(db *gorm.DB) error {
	// Check if roles already exist
	var count int64
	db.Model(&models.Role{}).Count(&count)
	if count > 0 {
		log.Println("Roles already seeded, skipping...")
		return nil
	}

	log.Println("Seeding default roles...")
	roles := []models.Role{
		{Name: "admin", Description: "Administrator with full access to all features"},
		{Name: "analyst", Description: "User who can analyze websites and view results"},
		{Name: "guest", Description: "Limited access user who can only view public analyses"},
	}

	return db.Create(&roles).Error
}

// SeedDefaultUsers seeds default admin and analyst users if they don't exist
func SeedDefaultUsers(db *gorm.DB) error {
	// Check if users already exist
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		log.Println("Users already seeded, skipping...")
		return nil
	}

	// Get roles
	var adminRole, analystRole models.Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return err
	}
	if err := db.Where("name = ?", "analyst").First(&analystRole).Error; err != nil {
		return err
	}

	log.Println("Seeding default users...")

	// Create admin user
	adminPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	adminUser := models.User{
		Username:     "admin",
		Email:        "admin@example.com",
		PasswordHash: string(adminPassword),
		RoleID:       adminRole.ID,
	}

	if err := db.Create(&adminUser).Error; err != nil {
		return err
	}

	// Create analyst user
	analystPassword, err := bcrypt.GenerateFromPassword([]byte("analyst123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	analystUser := models.User{
		Username:     "analyst",
		Email:        "analyst@example.com",
		PasswordHash: string(analystPassword),
		RoleID:       analystRole.ID,
	}

	return db.Create(&analystUser).Error
}

// SeedSampleWebsites seeds sample websites if they don't exist
func SeedSampleWebsites(db *gorm.DB) error {
	// Check if websites already exist
	var count int64
	db.Model(&models.Website{}).Count(&count)
	if count > 0 {
		log.Println("Websites already seeded, skipping...")
		return nil
	}

	log.Println("Seeding sample websites...")
	websites := []models.Website{
		{
			URL:         "https://example.com",
			Title:       "Example Website",
			Description: "A sample website for demonstration purposes",
		},
		{
			URL:         "https://demo.example.org",
			Title:       "Demo Website",
			Description: "Another sample website for testing",
		},
	}

	return db.Create(&websites).Error
}

// SeedSampleAnalysis seeds a sample analysis if none exist
func SeedSampleAnalysis(db *gorm.DB) error {
	// Check if analyses already exist
	var count int64
	db.Model(&models.Analysis{}).Count(&count)
	if count > 0 {
		log.Println("Analyses already seeded, skipping...")
		return nil
	}

	// Get first website and admin user
	var website models.Website
	var adminUser models.User

	if err := db.First(&website).Error; err != nil {
		return err
	}

	if err := db.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		return err
	}

	log.Println("Seeding sample analysis...")

	// Create a completed analysis
	analysis := models.Analysis{
		WebsiteID:   website.ID,
		UserID:      adminUser.ID,
		Status:      "completed",
		IsPublic:    true,
		StartedAt:   db.NowFunc(),
		CompletedAt: db.NowFunc(),
	}

	if err := db.Create(&analysis).Error; err != nil {
		return err
	}

	// Add some sample metrics
	metrics := []models.AnalysisMetric{
		{
			AnalysisID: analysis.ID,
			Category:   "seo",
			Name:       "seo_score",
			Value:      []byte(`{"score": 85, "max_score": 100}`),
		},
		{
			AnalysisID: analysis.ID,
			Category:   "performance",
			Name:       "performance_score",
			Value:      []byte(`{"score": 70, "max_score": 100}`),
		},
		{
			AnalysisID: analysis.ID,
			Category:   "accessibility",
			Name:       "accessibility_score",
			Value:      []byte(`{"score": 90, "max_score": 100}`),
		},
	}

	if err := db.Create(&metrics).Error; err != nil {
		return err
	}

	// Add sample recommendations
	recommendations := []models.Recommendation{
		{
			AnalysisID:  analysis.ID,
			Category:    "seo",
			Priority:    "high",
			Title:       "Add meta description",
			Description: "The page is missing a meta description. Add a concise, compelling meta description to improve click-through rates from search results.",
		},
		{
			AnalysisID:  analysis.ID,
			Category:    "performance",
			Priority:    "medium",
			Title:       "Optimize images",
			Description: "Several images are not optimized. Compress images to reduce page load time.",
			CodeSnippet: "<!-- Example image optimization suggestion -->\n<img src=\"image.jpg\" alt=\"Description\" width=\"800\" height=\"600\" loading=\"lazy\">",
		},
	}

	if err := db.Create(&recommendations).Error; err != nil {
		return err
	}

	// Add sample issues
	issues := []models.Issue{
		{
			AnalysisID:  analysis.ID,
			Category:    "seo",
			Severity:    "high",
			Title:       "Missing meta description",
			Description: "The page doesn't have a meta description tag.",
			Location:    "head section",
		},
		{
			AnalysisID:  analysis.ID,
			Category:    "performance",
			Severity:    "medium",
			Title:       "Large image files",
			Description: "Several images exceed the recommended file size of 200KB.",
			Location:    "homepage",
		},
	}

	return db.Create(&issues).Error
}

// SeedAll runs all seed functions
func SeedAll(db *gorm.DB) error {
	log.Println("Starting database seeding...")

	if err := SeedDefaultRoles(db); err != nil {
		return err
	}

	if err := SeedDefaultUsers(db); err != nil {
		return err
	}

	if err := SeedSampleWebsites(db); err != nil {
		return err
	}

	if err := SeedSampleAnalysis(db); err != nil {
		return err
	}

	log.Println("Database seeding completed successfully")
	return nil
}
