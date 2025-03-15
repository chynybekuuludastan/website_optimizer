// cmd/migrate/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/database/migration"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Define command-line flags
	migrateCmd := flag.Bool("migrate", false, "Run migrations")
	rollbackCmd := flag.Bool("rollback", false, "Rollback the last batch of migrations")
	resetCmd := flag.Bool("reset", false, "Rollback all migrations and re-run them")
	statusCmd := flag.Bool("status", false, "Show migration status")
	dsn := flag.String("dsn", os.Getenv("POSTGRES_URI"), "PostgreSQL connection string")

	// Parse command-line flags
	flag.Parse()

	// Check if at least one command was specified
	if !(*migrateCmd || *rollbackCmd || *resetCmd || *statusCmd) {
		flag.Usage()
		os.Exit(1)
	}

	// Connect to the database
	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	// Create migrator
	migrator := migration.NewMigrator(db)

	// Execute the command
	switch {
	case *migrateCmd:
		log.Println("Running migrations...")
		if err := migrator.Migrate(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations completed successfully")

	case *rollbackCmd:
		log.Println("Rolling back the last batch of migrations...")
		if err := migrator.Rollback(); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		log.Println("Rollback completed successfully")

	case *resetCmd:
		log.Println("Resetting all migrations...")
		if err := migrator.Reset(); err != nil {
			log.Fatalf("Reset failed: %v", err)
		}
		log.Println("Reset completed successfully")

	case *statusCmd:
		log.Println("Migration status:")
		status, err := migrator.GetStatus()
		if err != nil {
			log.Fatalf("Failed to get migration status: %v", err)
		}

		// Display status in a table format
		fmt.Println("+-----------------------+----------+-------+----------------------------+")
		fmt.Println("| Migration             | Applied? | Batch | Applied At                 |")
		fmt.Println("+-----------------------+----------+-------+----------------------------+")

		for _, s := range status {
			name := s["name"].(string)
			applied := s["applied"].(bool)
			batch := s["batch"].(int)

			// Format the timestamp properly
			var timestampStr string
			if applied {
				// Properly format the time.Time object
				if timestamp, ok := s["timestamp"].(time.Time); ok {
					timestampStr = timestamp.Format("2006-01-02 15:04:05")
				} else {
					timestampStr = "Invalid time format"
				}
			} else {
				timestampStr = "-"
			}

			appliedStr := "No"
			batchStr := "-"

			if applied {
				appliedStr = "Yes"
				batchStr = fmt.Sprintf("%d", batch)
			}

			fmt.Printf("| %-21s | %-8s | %-5s | %-26s |\n", name, appliedStr, batchStr, timestampStr)
		}

		fmt.Println("+-----------------------+----------+-------+----------------------------+")
	}
}
