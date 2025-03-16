package validation

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
)

// ContentValidator validates generated content
type ContentValidator struct {
	providers map[string]ValidationProvider
}

// ValidationProvider defines methods for validating content
type ValidationProvider interface {
	CheckPlagiarism(ctx context.Context, content string) (float64, error)
	CheckRelevance(ctx context.Context, content, topic string) (float64, error)
	CheckToneConsistency(ctx context.Context, original, improved string) (float64, error)
}

// ValidationResult contains the results of content validation
type ValidationResult struct {
	IsValid          bool     `json:"is_valid"`
	PlagiarismScore  float64  `json:"plagiarism_score"`  // 0-1 where 0 is completely unique
	RelevanceScore   float64  `json:"relevance_score"`   // 0-1 where 1 is highly relevant
	ConsistencyScore float64  `json:"consistency_score"` // 0-1 where 1 is highly consistent
	Issues           []string `json:"issues,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

// NewContentValidator creates a new content validator
func NewContentValidator() *ContentValidator {
	return &ContentValidator{
		providers: make(map[string]ValidationProvider),
	}
}

// RegisterProvider registers a validation provider
func (v *ContentValidator) RegisterProvider(name string, provider ValidationProvider) {
	v.providers[name] = provider
}

// Validate performs comprehensive validation of generated content
func (v *ContentValidator) Validate(ctx context.Context, original *llm.ContentRequest,
	improved *llm.ContentResponse) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid: true,
	}

	// Basic checks (always performed)
	v.performBasicChecks(original, improved, result)

	// Check if we have any providers for advanced validation
	if len(v.providers) > 0 {
		// Use the first provider (could implement provider selection logic)
		var provider ValidationProvider
		for _, p := range v.providers {
			provider = p
			break
		}

		// Check for plagiarism
		plagiarismScore, err := provider.CheckPlagiarism(ctx, improved.Content)
		if err == nil {
			result.PlagiarismScore = plagiarismScore
			if plagiarismScore > 0.3 { // 30% or more non-unique content
				result.IsValid = false
				result.Issues = append(result.Issues,
					fmt.Sprintf("Content contains %.1f%% potentially non-unique text", plagiarismScore*100))
			}
		}

		// Check for relevance to original topic
		topicHint := original.Title
		if original.AnalysisResults != nil && len(original.AnalysisResults.SEO) > 0 {
			// Extract keywords from SEO data
			// This is simplified; real implementation would be more sophisticated
			for _, issue := range original.AnalysisResults.SEO {
				if strings.Contains(strings.ToLower(issue.Description), "keyword") {
					topicHint += " " + issue.Description
					break
				}
			}
		}

		relevanceScore, err := provider.CheckRelevance(ctx, improved.Content, topicHint)
		if err == nil {
			result.RelevanceScore = relevanceScore
			if relevanceScore < 0.7 { // Less than 70% relevant
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Content may not be sufficiently relevant (%.1f%% relevance score)", relevanceScore*100))
				if relevanceScore < 0.5 {
					result.IsValid = false
					result.Issues = append(result.Issues, "Content lacks relevance to the original topic")
				}
			}
		}

		// Check tone consistency
		consistencyScore, err := provider.CheckToneConsistency(ctx, original.Content, improved.Content)
		if err == nil {
			result.ConsistencyScore = consistencyScore
			if consistencyScore < 0.6 { // Less than 60% consistent
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Tone may not be consistent with original (%.1f%% consistency)", consistencyScore*100))
			}
		}
	}

	return result, nil
}

// performBasicChecks runs simple validation rules that don't require external APIs
func (v *ContentValidator) performBasicChecks(original *llm.ContentRequest, improved *llm.ContentResponse, result *ValidationResult) {
	// Check for empty content
	if improved.Content == "" {
		result.IsValid = false
		result.Issues = append(result.Issues, "Generated content is empty")
	}

	// Check for minimum content length
	if len(improved.Content) < 50 && len(original.Content) > 100 {
		result.IsValid = false
		result.Issues = append(result.Issues, "Generated content is too short")
	}

	// Check for language consistency if specified
	if original.Language != "" && !v.isLanguageConsistent(improved.Content, original.Language) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Content may not be in the specified language (%s)", original.Language))
	}

	// Check for readability (simplified implementation)
	avgWordLength := v.calculateAvgWordLength(improved.Content)
	if avgWordLength > 7 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Content may be difficult to read (avg word length: %.1f)", avgWordLength))
	}
}

// isLanguageConsistent performs a basic check if content appears to be in specified language
// Note: This is a very simplified implementation
func (v *ContentValidator) isLanguageConsistent(content, language string) bool {
	// For proper implementation, use a language detection library
	// This is just a placeholder implementation
	switch language {
	case "en":
		// English typically uses Latin characters
		return v.hasLettersFromScript(content, unicode.Latin)
	case "ru":
		// Russian uses Cyrillic characters
		return v.hasLettersFromScript(content, unicode.Cyrillic)
	// Add other languages as needed
	default:
		return true // If we don't know how to check, assume it's consistent
	}
}

// hasLettersFromScript checks if text contains letters from a specific Unicode script
func (v *ContentValidator) hasLettersFromScript(text string, script *unicode.RangeTable) bool {
	letterCount := 0
	scriptLetters := 0

	for _, r := range text {
		if unicode.IsLetter(r) {
			letterCount++
			if unicode.Is(script, r) {
				scriptLetters++
			}
		}
	}

	// If at least 60% of letters are from the script, consider it matching
	if letterCount > 0 {
		return float64(scriptLetters)/float64(letterCount) >= 0.6
	}
	return false
}

// calculateAvgWordLength calculates the average word length in a text
func (v *ContentValidator) calculateAvgWordLength(text string) float64 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	totalLength := 0
	for _, word := range words {
		totalLength += len(word)
	}

	return float64(totalLength) / float64(len(words))
}
