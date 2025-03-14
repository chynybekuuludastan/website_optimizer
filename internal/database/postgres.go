package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
)

// DatabaseClient wraps the GORM DB connection
type DatabaseClient struct {
	*gorm.DB
}

// InitPostgreSQL initializes the PostgreSQL connection
func InitPostgreSQL(dsn string) (*DatabaseClient, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, err
	}

	// Set connection pool parameters
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Run migrations
	err = runMigrations(db)
	if err != nil {
		return nil, err
	}

	log.Println("Connected to PostgreSQL database")
	return &DatabaseClient{DB: db}, nil
}

// Close closes the database connection
func (d *DatabaseClient) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Auto-migrate models
	return db.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.Website{},
		&models.Analysis{},
		&models.AnalysisMetric{},
		&models.Recommendation{},
		&models.ContentImprovement{},
		&models.Issue{},
		&models.UserActivity{},
	)
}

// seedDefaultRoles seeds default roles if they don't exist
func seedDefaultRoles(db *gorm.DB) error {
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
