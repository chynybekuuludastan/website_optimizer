package llm

import (
	"encoding/json"

	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
)

// ExtractAnalysisResults processes raw analysis data into structured results
func ExtractAnalysisResults(metrics []models.AnalysisMetric) *AnalysisResults {
	results := &AnalysisResults{}

	// Process metrics by category
	for _, metric := range metrics {
		var data map[string]interface{}
		if err := json.Unmarshal(metric.Value, &data); err != nil {
			continue
		}

		switch metric.Category {
		case string(analyzer.SEOType):
			results.SEO = extractProblems(data, "seo")
			if score, ok := data["score"].(float64); ok {
				results.OverallScore += score * 0.3 // SEO has 30% weight
			}

		case string(analyzer.PerformanceType):
			results.Performance = extractProblems(data, "performance")
			if score, ok := data["score"].(float64); ok {
				results.OverallScore += score * 0.2 // Performance has 20% weight
			}

		case string(analyzer.AccessibilityType):
			results.Accessibility = extractProblems(data, "accessibility")
			if score, ok := data["score"].(float64); ok {
				results.OverallScore += score * 0.15 // Accessibility has 15% weight
			}

		case string(analyzer.MobileType):
			results.Mobile = extractProblems(data, "mobile")
			if score, ok := data["score"].(float64); ok {
				results.OverallScore += score * 0.15 // Mobile has 15% weight
			}

		case string(analyzer.ContentType):
			results.Content = extractProblems(data, "content")

			// Extract readability data
			if score, ok := data["flesch_reading_ease"].(float64); ok {
				results.ReadabilityScore = score
			}
			if level, ok := data["readability_level"].(string); ok {
				results.ReadabilityLevel = level
			}
			if count, ok := data["word_count"].(float64); ok {
				results.WordCount = int(count)
			} else if count, ok := data["word_count"].(int); ok {
				results.WordCount = count
			}
			if avg, ok := data["avg_words_per_sentence"].(float64); ok {
				results.AvgSentenceLen = avg
			}

			if score, ok := data["score"].(float64); ok {
				results.OverallScore += score * 0.2 // Content has 20% weight
			}
		}
	}

	// Normalize overall score to 0-100 range
	if results.OverallScore > 100 {
		results.OverallScore = 100
	} else if results.OverallScore < 0 {
		results.OverallScore = 0
	}

	return results
}

// extractProblems extracts analysis problems from raw analyzer data
func extractProblems(data map[string]interface{}, category string) []AnalysisProblem {
	var problems []AnalysisProblem

	// Extract issues from the data
	if issuesData, ok := data["issues"]; ok {
		issues := convertToIssueArray(issuesData)

		for _, issue := range issues {
			problem := AnalysisProblem{
				Type:        category,
				Severity:    "medium", // Default severity
				Description: "Unknown issue",
			}

			if t, ok := issue["type"].(string); ok {
				problem.Type = t
			}
			if sev, ok := issue["severity"].(string); ok {
				problem.Severity = sev
			}
			if desc, ok := issue["description"].(string); ok {
				problem.Description = desc
			}

			problems = append(problems, problem)
		}
	}

	// Extract recommendations and match with problems
	if recsData, ok := data["recommendations"]; ok {
		recommendations := convertToStringArray(recsData)

		// Match recommendations with problems if possible
		for i, rec := range recommendations {
			if i < len(problems) {
				problems[i].Recommendation = rec
			} else {
				// Create a new problem for remaining recommendations
				problems = append(problems, AnalysisProblem{
					Type:           category,
					Severity:       "low",
					Description:    "Suggestion for improvement",
					Recommendation: rec,
				})
			}
		}
	}

	return problems
}

// convertToIssueArray converts issues data to a standard format
func convertToIssueArray(issuesData interface{}) []map[string]interface{} {
	var issues []map[string]interface{}

	// Handle different types of issues data
	switch v := issuesData.(type) {
	case []map[string]interface{}:
		return v
	case []interface{}:
		for _, issue := range v {
			if issueMap, ok := issue.(map[string]interface{}); ok {
				issues = append(issues, issueMap)
			}
		}
	case map[string]interface{}:
		// If issues are presented as a map, convert to an array
		for key, value := range v {
			if issueMap, ok := value.(map[string]interface{}); ok {
				issueMap["type"] = key
				issues = append(issues, issueMap)
			}
		}
	}

	return issues
}

// convertToStringArray converts recommendations data to string array
func convertToStringArray(recsData interface{}) []string {
	var recs []string

	// Handle different types of recommendations data
	switch v := recsData.(type) {
	case []string:
		return v
	case []interface{}:
		for _, rec := range v {
			if recStr, ok := rec.(string); ok {
				recs = append(recs, recStr)
			}
		}
	}

	return recs
}
