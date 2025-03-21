// internal/models/models.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Role represents a user role in the system
type Role struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(50);unique;not null;index"`
	Description string `gorm:"type:text"`
	// Relationships
	Users []User `gorm:"foreignKey:RoleID"`
}

// User represents a system user
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Username     string         `gorm:"type:varchar(100);unique;not null;index"`
	Email        string         `gorm:"type:varchar(255);unique;not null;index"`
	PasswordHash string         `gorm:"type:varchar(255);not null"`
	RoleID       uint           `gorm:"not null;index"`
	Role         Role           `gorm:"foreignKey:RoleID"`
	CreatedAt    time.Time      `gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	// Relationships
	Analyses       []Analysis     `gorm:"foreignKey:UserID"`
	UserActivities []UserActivity `gorm:"foreignKey:UserID"`
}

// Website represents a website that was analyzed
type Website struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	URL         string         `gorm:"type:varchar(2048);not null;index"`
	Title       string         `gorm:"type:varchar(255);index"`
	Description string         `gorm:"type:text"`
	CreatedAt   time.Time      `gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	// Relationships
	Analyses []Analysis `gorm:"foreignKey:WebsiteID"`
}

// Analysis represents a website analysis
type Analysis struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WebsiteID   uuid.UUID      `gorm:"type:uuid;not null;index"`
	Website     Website        `gorm:"foreignKey:WebsiteID"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
	User        User           `gorm:"foreignKey:UserID"`
	Status      string         `gorm:"type:varchar(50);not null;default:'pending';index"`
	StartedAt   time.Time      `gorm:"default:null;index"`
	CompletedAt time.Time      `gorm:"default:null;index"`
	IsPublic    bool           `gorm:"default:false;index"`
	Metadata    datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt   time.Time      `gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	// Relationships
	Metrics             []AnalysisMetric     `gorm:"foreignKey:AnalysisID"`
	Recommendations     []Recommendation     `gorm:"foreignKey:AnalysisID"`
	ContentImprovements []ContentImprovement `gorm:"foreignKey:AnalysisID"`
	Issues              []Issue              `gorm:"foreignKey:AnalysisID"`
}

// AnalysisMetric represents a metric from the analysis
type AnalysisMetric struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Analysis   Analysis       `gorm:"foreignKey:AnalysisID"`
	Category   string         `gorm:"type:varchar(100);not null;index"`
	Name       string         `gorm:"type:varchar(100);not null;index"`
	Value      datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
}

type Issue struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;not null;index" json:"analysis_id"`
	Analysis    Analysis  `gorm:"foreignKey:AnalysisID;references:ID" json:"analysis"`
	Category    string    `gorm:"type:varchar(100);not null;index" json:"category"`
	Severity    string    `gorm:"type:varchar(50);not null;index" json:"severity"` // high, medium, low
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	Location    string    `gorm:"type:text" json:"location"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type Recommendation struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;not null;index" json:"analysis_id"`
	Analysis    Analysis  `gorm:"foreignKey:AnalysisID;references:ID" json:"analysis"`
	Category    string    `gorm:"type:varchar(100);not null;index" json:"category"`
	Priority    string    `gorm:"type:varchar(50);not null;index" json:"priority"` // high, medium, low
	Title       string    `gorm:"type:text;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	CodeSnippet string    `gorm:"type:text" json:"code_snippet"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type ContentImprovement struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AnalysisID      uuid.UUID `gorm:"type:uuid;not null;index" json:"analysis_id"`
	Analysis        Analysis  `gorm:"foreignKey:AnalysisID;references:ID" json:"analysis"`
	ElementType     string    `gorm:"type:varchar(50);not null;index" json:"element_type"` // heading, cta, content, etc.
	OriginalContent string    `gorm:"type:text" json:"original_content"`
	ImprovedContent string    `gorm:"type:text" json:"improved_content"`
	LLMModel        string    `gorm:"type:varchar(100)" json:"llm_model"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// UserActivity logs user actions in the system
type UserActivity struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	User       User           `gorm:"foreignKey:UserID"`
	ActionType string         `gorm:"type:varchar(100);not null;index"` // login, analyze, etc.
	EntityType string         `gorm:"type:varchar(100);index"`          // website, analysis, etc.
	EntityID   uuid.UUID      `gorm:"type:uuid;index"`
	Details    datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"autoCreateTime;index"`
}
