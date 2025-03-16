package llm

import (
	"time"
)

// ContentRequest represents a request for content improvement
type ContentRequest struct {
	URL             string           `json:"url"`                        // The URL of the page to improve
	Title           string           `json:"title"`                      // Current page title/heading
	CTAText         string           `json:"cta_text"`                   // Current call-to-action text
	Content         string           `json:"content"`                    // Current page content
	Language        string           `json:"language,omitempty"`         // Optional language for localized improvements
	AnalysisResults *AnalysisResults `json:"analysis_results,omitempty"` // Results from analyzers
	TargetAudience  string           `json:"target_audience,omitempty"`  // Target audience if available
}

// ContentResponse represents the response with improved content
type ContentResponse struct {
	Title          string        `json:"heading"`                   // Improved heading
	CTAText        string        `json:"cta_button"`                // Improved CTA button text
	Content        string        `json:"improved_content"`          // Improved content
	HTML           string        `json:"html,omitempty"`            // Optional HTML representation
	ProviderUsed   string        `json:"provider_used,omitempty"`   // Provider that generated the content
	CachedResult   bool          `json:"cached_result"`             // Whether this came from cache
	ProcessingTime time.Duration `json:"processing_time,omitempty"` // How long it took to generate
}

// AnalysisResults represents aggregated results from different analyzers
type AnalysisResults struct {
	SEO              []AnalysisProblem `json:"seo,omitempty"`
	Performance      []AnalysisProblem `json:"performance,omitempty"`
	Accessibility    []AnalysisProblem `json:"accessibility,omitempty"`
	Mobile           []AnalysisProblem `json:"mobile,omitempty"`
	Content          []AnalysisProblem `json:"content,omitempty"`
	ReadabilityScore float64           `json:"readability_score,omitempty"`
	ReadabilityLevel string            `json:"readability_level,omitempty"`
	WordCount        int               `json:"word_count,omitempty"`
	AvgSentenceLen   float64           `json:"avg_sentence_length,omitempty"`
	OverallScore     float64           `json:"overall_score,omitempty"`
}

// AnalysisProblem represents a single issue found by an analyzer
type AnalysisProblem struct {
	Type           string `json:"type"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation,omitempty"`
}
