package migration

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Migration represents a database migration record
type Migration struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"type:varchar(255);not null;unique"`
	Batch     int       `gorm:"not null"`
	AppliedAt time.Time `gorm:"autoCreateTime"`
}

// MigrationFunc defines a function that can run a migration
type MigrationFunc func(tx *gorm.DB) error

// Migrator handles database migrations
type Migrator struct {
	DB         *gorm.DB
	Migrations map[string]struct {
		Up   MigrationFunc
		Down MigrationFunc
	}
	CurrentBatch int
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB) *Migrator {
	// Ensure migrations table exists
	if err := db.AutoMigrate(&Migration{}); err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	// Get current batch number
	var maxBatch int
	db.Model(&Migration{}).Select("COALESCE(MAX(batch), 0)").Row().Scan(&maxBatch)

	return &Migrator{
		DB:           db,
		Migrations:   RegisterMigrations(),
		CurrentBatch: maxBatch + 1,
	}
}

// RegisterMigrations registers all migrations with up and down functions
func RegisterMigrations() map[string]struct {
	Up   MigrationFunc
	Down MigrationFunc
} {
	return map[string]struct {
		Up   MigrationFunc
		Down MigrationFunc
	}{
		"01_create_roles_table": {
			Up:   CreateRolesTable,
			Down: DropRolesTable,
		},
		"02_create_users_table": {
			Up:   CreateUsersTable,
			Down: DropUsersTable,
		},
		"03_create_websites_table": {
			Up:   CreateWebsitesTable,
			Down: DropWebsitesTable,
		},
		"04_create_analysis_table": {
			Up:   CreateAnalysisTable,
			Down: DropAnalysisTable,
		},
		"05_create_analysis_metrics_table": {
			Up:   CreateAnalysisMetricsTable,
			Down: DropAnalysisMetricsTable,
		},
		"06_create_recommendations_table": {
			Up:   CreateRecommendationsTable,
			Down: DropRecommendationsTable,
		},
		"07_create_content_improvements_table": {
			Up:   CreateContentImprovementsTable,
			Down: DropContentImprovementsTable,
		},
		"08_create_issues_table": {
			Up:   CreateIssuesTable,
			Down: DropIssuesTable,
		},
		"09_create_user_activity_table": {
			Up:   CreateUserActivityTable,
			Down: DropUserActivityTable,
		},
		"10_add_indexes": {
			Up:   AddIndexes,
			Down: RemoveIndexes,
		},
		"11_alter_recommendation_title_column": {
			Up:   AddAlterRecommendationTitleColumn,
			Down: RollbackAlterRecommendationTitleColumn,
		},
	}
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	// Get already applied migrations
	var appliedMigrations []Migration
	if err := m.DB.Find(&appliedMigrations).Error; err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a map of applied migrations for quick lookup
	appliedMap := make(map[string]bool)
	for _, migration := range appliedMigrations {
		appliedMap[migration.Name] = true
	}

	// Run pending migrations in order
	for name, migration := range m.Migrations {
		if !appliedMap[name] {
			log.Printf("Running migration: %s", name)

			// Start a transaction
			err := m.DB.Transaction(func(tx *gorm.DB) error {
				// Run the migration
				if err := migration.Up(tx); err != nil {
					return fmt.Errorf("migration failed: %w", err)
				}

				// Record the migration
				return tx.Create(&Migration{
					Name:  name,
					Batch: m.CurrentBatch,
				}).Error
			})

			if err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", name, err)
			}

			log.Printf("Migration applied: %s", name)
		}
	}

	return nil
}

// Rollback rolls back the last batch of migrations
func (m *Migrator) Rollback() error {
	// Get migrations from the last batch
	var migrationsToRollback []Migration
	if err := m.DB.Where("batch = ?", m.CurrentBatch-1).Order("id DESC").Find(&migrationsToRollback).Error; err != nil {
		return fmt.Errorf("failed to get migrations to rollback: %w", err)
	}

	if len(migrationsToRollback) == 0 {
		log.Println("No migrations to rollback")
		return nil
	}

	// Roll back each migration
	for _, migration := range migrationsToRollback {
		if migrationFuncs, ok := m.Migrations[migration.Name]; ok {
			log.Printf("Rolling back migration: %s", migration.Name)

			// Start a transaction
			err := m.DB.Transaction(func(tx *gorm.DB) error {
				// Run the down migration
				if err := migrationFuncs.Down(tx); err != nil {
					return fmt.Errorf("rollback failed: %w", err)
				}

				// Remove the migration record
				return tx.Delete(&migration).Error
			})

			if err != nil {
				return fmt.Errorf("failed to rollback migration %s: %w", migration.Name, err)
			}

			log.Printf("Migration rolled back: %s", migration.Name)
		}
	}

	return nil
}

// Reset rolls back all migrations and then applies them again
func (m *Migrator) Reset() error {
	// Get all applied migrations
	var appliedMigrations []Migration
	if err := m.DB.Order("id DESC").Find(&appliedMigrations).Error; err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Roll back all migrations
	for _, migration := range appliedMigrations {
		if migrationFuncs, ok := m.Migrations[migration.Name]; ok {
			log.Printf("Rolling back migration: %s", migration.Name)

			// Start a transaction
			err := m.DB.Transaction(func(tx *gorm.DB) error {
				// Run the down migration
				if err := migrationFuncs.Down(tx); err != nil {
					return fmt.Errorf("rollback failed: %w", err)
				}

				// Remove the migration record
				return tx.Delete(&migration).Error
			})

			if err != nil {
				return fmt.Errorf("failed to rollback migration %s: %w", migration.Name, err)
			}
		}
	}

	// Reset batch number
	m.CurrentBatch = 1

	// Apply all migrations
	return m.Migrate()
}

// GetStatus returns the status of all migrations
func (m *Migrator) GetStatus() ([]map[string]interface{}, error) {
	// Get all applied migrations
	var appliedMigrations []Migration
	if err := m.DB.Find(&appliedMigrations).Error; err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a map of applied migrations
	appliedMap := make(map[string]Migration)
	for _, migration := range appliedMigrations {
		appliedMap[migration.Name] = migration
	}

	// Create status list
	var status []map[string]interface{}
	for name := range m.Migrations {
		migration, applied := appliedMap[name]

		// Initialize the status map
		statusMap := map[string]interface{}{
			"name":    name,
			"applied": applied,
		}

		// Set batch and timestamp based on whether migration is applied
		if applied {
			statusMap["batch"] = migration.Batch
			statusMap["timestamp"] = migration.AppliedAt
		} else {
			statusMap["batch"] = 0
			statusMap["timestamp"] = time.Time{}
		}

		status = append(status, statusMap)
	}

	return status, nil
}

// AddAlterRecommendationTitleColumn changes the Title column in recommendations table from varchar to text
func AddAlterRecommendationTitleColumn(tx *gorm.DB) error {
	// Check if the table exists first
	var exists bool
	err := tx.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'recommendations')").Scan(&exists).Error
	if err != nil {
		return err
	}

	// Only alter if the table exists
	if exists {
		// First check if the column is varchar(255)
		var columnType string
		err := tx.Raw("SELECT data_type FROM information_schema.columns WHERE table_name = 'recommendations' AND column_name = 'title'").Scan(&columnType).Error
		if err != nil {
			return err
		}

		// If the column type contains 'character' (meaning it's a varchar), alter it
		if columnType == "character varying" {
			return tx.Exec("ALTER TABLE recommendations ALTER COLUMN title TYPE TEXT").Error
		}
	}

	return nil
}

// RollbackAlterRecommendationTitleColumn is a no-op since we don't want to convert back to varchar
func RollbackAlterRecommendationTitleColumn(tx *gorm.DB) error {
	// This is intentionally a no-op as we don't want to reduce the column size
	// and potentially truncate data
	return nil
}
