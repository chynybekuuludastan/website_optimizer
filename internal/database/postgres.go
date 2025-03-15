// internal/database/postgres.go
package database

import (
	"log"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/database/migration"
	"github.com/chynybekuuludastan/website_optimizer/internal/database/seed"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseClient wraps the GORM DB connection
type DatabaseClient struct {
	*gorm.DB
}

// InitPostgreSQL initializes the PostgreSQL connection
func InitPostgreSQL(dsn string) (*DatabaseClient, error) {
	// Create database connection
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
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

	// Create client
	client := &DatabaseClient{DB: db}

	// Run migrations
	log.Println("Running database migrations...")
	migrator := migration.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		log.Printf("Migration warning: %v", err)
	}

	// Seed default data
	log.Println("Seeding default data...")
	if err := seed.SeedDefaultRoles(db); err != nil {
		log.Printf("Seed warning: %v", err)
	}
	if err := seed.SeedDefaultUsers(db); err != nil {
		log.Printf("Seed warning: %v", err)
	}

	log.Println("Database setup completed")
	return client, nil
}

// Close closes the database connection
func (d *DatabaseClient) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WithTransaction executes a function within a transaction
func (d *DatabaseClient) WithTransaction(fn func(*gorm.DB) error) error {
	return d.Transaction(fn)
}
