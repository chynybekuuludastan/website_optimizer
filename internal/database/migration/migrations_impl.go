// internal/database/migration/migrations_impl.go
package migration

import (
	"strings"

	"gorm.io/gorm"
)

// CreateRolesTable creates the roles table
func CreateRolesTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS roles (
			id SERIAL PRIMARY KEY,
			name VARCHAR(50) NOT NULL UNIQUE,
			description TEXT
		)
	`).Error
}

// DropRolesTable drops the roles table
func DropRolesTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS roles CASCADE").Error
}

// CreateUsersTable creates the users table
func CreateUsersTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(100) NOT NULL UNIQUE,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role_id INTEGER NOT NULL REFERENCES roles(id),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		)
	`).Error
}

// DropUsersTable drops the users table
func DropUsersTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS users CASCADE").Error
}

// CreateWebsitesTable creates the websites table
func CreateWebsitesTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS websites (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			url VARCHAR(2048) NOT NULL,
			title VARCHAR(255),
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		)
	`).Error
}

// DropWebsitesTable drops the websites table
func DropWebsitesTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS websites CASCADE").Error
}

// CreateAnalysisTable creates the analysis table
func CreateAnalysisTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS analysis (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			website_id UUID NOT NULL REFERENCES websites(id),
			user_id UUID NOT NULL REFERENCES users(id),
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			started_at TIMESTAMP WITH TIME ZONE,
			completed_at TIMESTAMP WITH TIME ZONE,
			is_public BOOLEAN NOT NULL DEFAULT FALSE,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		)
	`).Error
}

// DropAnalysisTable drops the analysis table
func DropAnalysisTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS analysis CASCADE").Error
}

// CreateAnalysisMetricsTable creates the analysis_metrics table
func CreateAnalysisMetricsTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS analysis_metrics (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			analysis_id UUID NOT NULL REFERENCES analysis(id),
			category VARCHAR(100) NOT NULL,
			name VARCHAR(100) NOT NULL,
			value JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}

// DropAnalysisMetricsTable drops the analysis_metrics table
func DropAnalysisMetricsTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS analysis_metrics CASCADE").Error
}

// CreateRecommendationsTable creates the recommendations table
func CreateRecommendationsTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS recommendations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			analysis_id UUID NOT NULL REFERENCES analysis(id),
			category VARCHAR(100) NOT NULL,
			priority VARCHAR(50) NOT NULL,
			title TEXT NOT NULL,       -- Changed from VARCHAR(255) to TEXT
			description TEXT,
			code_snippet TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}

// DropRecommendationsTable drops the recommendations table
func DropRecommendationsTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS recommendations CASCADE").Error
}

// CreateContentImprovementsTable creates the content_improvements table
func CreateContentImprovementsTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS content_improvements (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			analysis_id UUID NOT NULL REFERENCES analysis(id),
			element_type VARCHAR(50) NOT NULL,
			original_content TEXT,
			improved_content TEXT,
			llm_model VARCHAR(100),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}

// DropContentImprovementsTable drops the content_improvements table
func DropContentImprovementsTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS content_improvements CASCADE").Error
}

// CreateIssuesTable creates the issues table
func CreateIssuesTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS issues (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			analysis_id UUID NOT NULL REFERENCES analysis(id),
			category VARCHAR(100) NOT NULL,
			severity VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			location TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}

// DropIssuesTable drops the issues table
func DropIssuesTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS issues CASCADE").Error
}

// CreateUserActivityTable creates the user_activity table
func CreateUserActivityTable(tx *gorm.DB) error {
	return tx.Exec(`
		CREATE TABLE IF NOT EXISTS user_activity (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id),
			action_type VARCHAR(100) NOT NULL,
			entity_type VARCHAR(100),
			entity_id UUID,
			details JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
}

// DropUserActivityTable drops the user_activity table
func DropUserActivityTable(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS user_activity CASCADE").Error
}

// AddIndexes adds indexes to improve query performance
func AddIndexes(tx *gorm.DB) error {
	// Users indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)").Error; err != nil {
		return err
	}

	// Websites indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_websites_url ON websites(url)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_websites_title ON websites(title)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_websites_created_at ON websites(created_at)").Error; err != nil {
		return err
	}

	// Analysis indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_website_id ON analysis(website_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_user_id ON analysis(user_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_status ON analysis(status)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_is_public ON analysis(is_public)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_created_at ON analysis(created_at)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_completed_at ON analysis(completed_at)").Error; err != nil {
		return err
	}

	// Composite indexes for common queries
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_user_status ON analysis(user_id, status)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_website_status ON analysis(website_id, status)").Error; err != nil {
		return err
	}

	// Analysis metrics indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_metrics_analysis_id ON analysis_metrics(analysis_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_metrics_category ON analysis_metrics(category)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_metrics_name ON analysis_metrics(name)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_analysis_metrics_category_name ON analysis_metrics(category, name)").Error; err != nil {
		return err
	}

	// Recommendations indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_recommendations_analysis_id ON recommendations(analysis_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_recommendations_category ON recommendations(category)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_recommendations_priority ON recommendations(priority)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_recommendations_analysis_priority ON recommendations(analysis_id, priority)").Error; err != nil {
		return err
	}

	// Content improvements indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_content_improvements_analysis_id ON content_improvements(analysis_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_content_improvements_element_type ON content_improvements(element_type)").Error; err != nil {
		return err
	}

	// Issues indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_issues_analysis_id ON issues(analysis_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_issues_category ON issues(category)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_issues_severity ON issues(severity)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_issues_analysis_severity ON issues(analysis_id, severity)").Error; err != nil {
		return err
	}

	// User activity indexes
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_user_activity_user_id ON user_activity(user_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_user_activity_action_type ON user_activity(action_type)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_user_activity_entity_id ON user_activity(entity_id)").Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_user_activity_created_at ON user_activity(created_at)").Error; err != nil {
		return err
	}

	return nil
}

// RemoveIndexes removes the indexes created by AddIndexes
func RemoveIndexes(tx *gorm.DB) error {
	// Drop all the created indexes
	indexes := []string{
		"idx_users_role_id", "idx_users_email", "idx_users_username", "idx_users_created_at",
		"idx_websites_url", "idx_websites_title", "idx_websites_created_at",
		"idx_analysis_website_id", "idx_analysis_user_id", "idx_analysis_status",
		"idx_analysis_is_public", "idx_analysis_created_at", "idx_analysis_completed_at",
		"idx_analysis_user_status", "idx_analysis_website_status",
		"idx_analysis_metrics_analysis_id", "idx_analysis_metrics_category",
		"idx_analysis_metrics_name", "idx_analysis_metrics_category_name",
		"idx_recommendations_analysis_id", "idx_recommendations_category",
		"idx_recommendations_priority", "idx_recommendations_analysis_priority",
		"idx_content_improvements_analysis_id", "idx_content_improvements_element_type",
		"idx_issues_analysis_id", "idx_issues_category", "idx_issues_severity",
		"idx_issues_analysis_severity",
		"idx_user_activity_user_id", "idx_user_activity_action_type",
		"idx_user_activity_entity_id", "idx_user_activity_created_at",
	}

	for _, index := range indexes {
		// Handle each table separately
		if strings.HasPrefix(index, "idx_users_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_websites_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_analysis_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_analysis_metrics_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_recommendations_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_content_improvements_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_issues_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		} else if strings.HasPrefix(index, "idx_user_activity_") {
			if err := tx.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
