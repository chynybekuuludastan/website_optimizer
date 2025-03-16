package prompts

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
)

// Generator creates prompts for LLM services
type Generator struct{}

// NewGenerator creates a new prompt generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateContentPrompt creates a prompt for content improvement
func (g *Generator) GenerateContentPrompt(request *llm.ContentRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert in web content optimization.\n\n")

	// Base information about the website
	sb.WriteString(fmt.Sprintf("Based on the analysis of the website %s, suggest improved versions of headings, CTA buttons, and text content to increase conversion rates.", request.URL))

	// Add analysis results if available
	if request.AnalysisResults != nil {
		sb.WriteString(" Address the following issues:\n\n")

		// Add SEO issues
		if len(request.AnalysisResults.SEO) > 0 {
			sb.WriteString("SEO Issues:\n")
			issues := prioritizeIssues(request.AnalysisResults.SEO, 3)
			for _, issue := range issues {
				sb.WriteString(fmt.Sprintf("- %s (%s severity)\n", issue.Description, issue.Severity))
			}
			sb.WriteString("\n")
		}

		// Add Content issues
		if len(request.AnalysisResults.Content) > 0 {
			sb.WriteString("Content Issues:\n")
			issues := prioritizeIssues(request.AnalysisResults.Content, 3)
			for _, issue := range issues {
				sb.WriteString(fmt.Sprintf("- %s (%s severity)\n", issue.Description, issue.Severity))
			}
			sb.WriteString("\n")
		}

		// Add Readability info
		if request.AnalysisResults.ReadabilityScore > 0 {
			sb.WriteString(fmt.Sprintf("Readability: Score %.1f", request.AnalysisResults.ReadabilityScore))
			if request.AnalysisResults.ReadabilityLevel != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", request.AnalysisResults.ReadabilityLevel))
			}
			if request.AnalysisResults.WordCount > 0 {
				sb.WriteString(fmt.Sprintf(", %d words", request.AnalysisResults.WordCount))
			}
			if request.AnalysisResults.AvgSentenceLen > 0 {
				sb.WriteString(fmt.Sprintf(", avg. %.1f words per sentence", request.AnalysisResults.AvgSentenceLen))
			}
			sb.WriteString("\n\n")
		}
	}

	// Add target audience if available
	if request.TargetAudience != "" {
		sb.WriteString(fmt.Sprintf("Target Audience: %s\n\n", request.TargetAudience))
	}

	// Add current content information
	sb.WriteString("Consider the current content:\n")
	sb.WriteString(fmt.Sprintf("- Heading: \"%s\"\n", request.Title))
	sb.WriteString(fmt.Sprintf("- CTA: \"%s\"\n", request.CTAText))
	sb.WriteString(fmt.Sprintf("- Text: \"%s\"\n\n", request.Content))

	// Add language specification if available
	if request.Language != "" && request.Language != "en" {
		sb.WriteString(fmt.Sprintf("Please provide your response in %s language.\n\n", request.Language))
	}

	// Request format
	sb.WriteString("Response format: JSON with fields 'heading', 'cta_button', 'improved_content'.\n")
	sb.WriteString("Do not include any explanations, just return the JSON object.")

	return sb.String()
}

// GenerateHTMLPrompt creates a prompt for HTML generation
func (g *Generator) GenerateHTMLPrompt(originalContent string, improved *llm.ContentResponse) string {
	var sb strings.Builder

	sb.WriteString("You are an expert HTML developer. Create clean, semantic HTML code.\n\n")

	// Make sure we have valid data to work with
	title := improved.Title
	if title == "" {
		title = "Heading"
	}

	ctaText := improved.CTAText
	if ctaText == "" {
		ctaText = "Call to Action"
	}

	content := improved.Content
	if content == "" {
		content = originalContent
	}

	// Format the JSON for improvements
	improvementsJSON, _ := json.Marshal(map[string]string{
		"heading":          title,
		"cta_button":       ctaText,
		"improved_content": content,
	})

	sb.WriteString("Based on the original content and the suggested improvements, create an updated HTML code.\n\n")

	sb.WriteString("Original content:\n")
	sb.WriteString(originalContent)
	sb.WriteString("\n\n")

	sb.WriteString("Suggested improvements (JSON):\n")
	sb.WriteString(string(improvementsJSON))
	sb.WriteString("\n\n")

	sb.WriteString("Return only the HTML code without any explanations or markdown formatting. Do not include backticks or 'html' language tags.\n")
	sb.WriteString("Use modern, semantic HTML5 with clean structure. Focus on creating a user-friendly, accessible, and SEO-optimized structure.\n")

	return sb.String()
}

// prioritizeIssues prioritizes issues based on severity
func prioritizeIssues(issues []llm.AnalysisProblem, maxIssues int) []llm.AnalysisProblem {
	// Create a copy to avoid modifying the original
	issuesCopy := make([]llm.AnalysisProblem, len(issues))
	copy(issuesCopy, issues)

	// Sort by severity: high, medium, low
	sort.Slice(issuesCopy, func(i, j int) bool {
		return getSeverityRank(issuesCopy[i].Severity) > getSeverityRank(issuesCopy[j].Severity)
	})

	// Limit the number of issues
	if len(issuesCopy) > maxIssues {
		issuesCopy = issuesCopy[:maxIssues]
	}

	return issuesCopy
}

// getSeverityRank returns a numeric representation of severity for sorting
func getSeverityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
