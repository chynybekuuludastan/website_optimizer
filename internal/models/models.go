package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Role represents a user role in the system
type Role struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"type:varchar(50);unique;not null"`
	Description string `gorm:"type:text"`
}

// User represents a system user
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Username     string         `gorm:"type:varchar(100);unique;not null"`
	Email        string         `gorm:"type:varchar(255);unique;not null"`
	PasswordHash string         `gorm:"type:varchar(255);not null"`
	RoleID       uint           `gorm:"not null"`
	Role         Role           `gorm:"foreignKey:RoleID"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// Website represents a website that was analyzed
type Website struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	URL         string         `gorm:"type:varchar(2048);not null"`
	Title       string         `gorm:"type:varchar(255)"`
	Description string         `gorm:"type:text"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// Analysis represents a website analysis
type Analysis struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WebsiteID   uuid.UUID      `gorm:"type:uuid;not null"`
	Website     Website        `gorm:"foreignKey:WebsiteID"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null"`
	User        User           `gorm:"foreignKey:UserID"`
	Status      string         `gorm:"type:varchar(50);not null;default:'pending'"`
	StartedAt   time.Time      `gorm:"default:null"`
	CompletedAt time.Time      `gorm:"default:null"`
	IsPublic    bool           `gorm:"default:false"`
	Metadata    datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// AnalysisMetric represents a metric from the analysis
type AnalysisMetric struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID uuid.UUID      `gorm:"type:uuid;not null"`
	Analysis   Analysis       `gorm:"foreignKey:AnalysisID"`
	Category   string         `gorm:"type:varchar(100);not null"`
	Name       string         `gorm:"type:varchar(100);not null"`
	Value      datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
}

// Recommendation represents a recommendation for website improvement
type Recommendation struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;not null"`
	Analysis    Analysis  `gorm:"foreignKey:AnalysisID"`
	Category    string    `gorm:"type:varchar(100);not null"`
	Priority    string    `gorm:"type:varchar(50);not null"`
	Title       string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	CodeSnippet string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// ContentImprovement represents LLM-generated content improvements
type ContentImprovement struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID      uuid.UUID `gorm:"type:uuid;not null"`
	Analysis        Analysis  `gorm:"foreignKey:AnalysisID"`
	ElementType     string    `gorm:"type:varchar(50);not null"`
	OriginalContent string    `gorm:"type:text"`
	ImprovedContent string    `gorm:"type:text"`
	LLMModel        string    `gorm:"type:varchar(100)"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
}

// Issue represents a problem found during analysis
type Issue struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;not null"`
	Analysis    Analysis  `gorm:"foreignKey:AnalysisID"`
	Category    string    `gorm:"type:varchar(100);not null"`
	Severity    string    `gorm:"type:varchar(50);not null"`
	Title       string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	Location    string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// UserActivity logs user actions in the system
type UserActivity struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null"`
	User       User           `gorm:"foreignKey:UserID"`
	ActionType string         `gorm:"type:varchar(100);not null"`
	EntityType string         `gorm:"type:varchar(100)"`
	EntityID   uuid.UUID      `gorm:"type:uuid"`
	Details    datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
}
