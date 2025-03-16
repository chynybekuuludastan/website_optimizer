package llm

import (
	"time"
)

// Extend ContentResponse with feedback and validation fields
type EnhancedContentResponse struct {
	ContentResponse                         // Embed the base response
	PromptID         string                 `json:"prompt_id,omitempty"`  // ID for feedback
	ValidationResult map[string]interface{} `json:"validation,omitempty"` // Validation results
	Meta             map[string]interface{} `json:"meta,omitempty"`       // Additional metadata
}

// Additional models for specialized content types
type MetaDescriptionResponse struct {
	MetaDescription string        `json:"meta_description"` // The generated meta description
	PromptID        string        `json:"prompt_id,omitempty"`
	ProviderUsed    string        `json:"provider_used,omitempty"`
	ProcessingTime  time.Duration `json:"processing_time,omitempty"`
	CachedResult    bool          `json:"cached_result"`
}

type HeadingResponse struct {
	Heading        string        `json:"heading"` // The generated heading
	PromptID       string        `json:"prompt_id,omitempty"`
	ProviderUsed   string        `json:"provider_used,omitempty"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`
	CachedResult   bool          `json:"cached_result"`
}

type CTAResponse struct {
	CTAText        string        `json:"cta_button"` // The generated CTA text
	PromptID       string        `json:"prompt_id,omitempty"`
	ProviderUsed   string        `json:"provider_used,omitempty"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`
	CachedResult   bool          `json:"cached_result"`
}
