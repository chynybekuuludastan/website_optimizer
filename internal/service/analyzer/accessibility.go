package analyzer

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AccessibilityResult contains the accessibility analysis results
type AccessibilityResult struct {
	Score              float64                  `json:"score"`
	MissingAltText     []string                 `json:"missing_alt_text"`
	MissingLabelForms  int                      `json:"missing_label_forms"`
	ContrastIssues     []map[string]interface{} `json:"contrast_issues"`
	AriaAttributesUsed bool                     `json:"aria_attributes_used"`
	SemanticHTMLUsed   bool                     `json:"semantic_html_used"`
	HasSkipLinks       bool                     `json:"has_skip_links"`
	TabindexIssues     []string                 `json:"tabindex_issues"`
	AccessibleForms    bool                     `json:"accessible_forms"`
	Issues             []map[string]interface{} `json:"issues"`
	Recommendations    []string                 `json:"recommendations"`
}

// AnalyzeAccessibility performs accessibility analysis on the website data
func AnalyzeAccessibility(data *parser.WebsiteData) map[string]interface{} {
	result := AccessibilityResult{
		MissingAltText:  []string{},
		ContrastIssues:  []map[string]interface{}{},
		TabindexIssues:  []string{},
		Issues:          []map[string]interface{}{},
		Recommendations: []string{},
	}

	// Check for missing alt text on images
	for _, img := range data.Images {
		if img.Alt == "" {
			result.MissingAltText = append(result.MissingAltText, img.URL)
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "missing_alt_text",
				"severity":    "high",
				"description": "Image is missing alt text",
				"url":         img.URL,
			})
		}
	}
	if len(result.MissingAltText) > 0 {
		result.Recommendations = append(result.Recommendations, "Add descriptive alt text to all images")
	}

	// Parse HTML for accessibility checks
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
	if err == nil {
		// Check for forms with missing labels
		formFields := doc.Find("input, select, textarea").Length()
		labeledFields := doc.Find("label[for], input[aria-label], select[aria-label], textarea[aria-label]").Length()
		result.MissingLabelForms = formFields - labeledFields

		if result.MissingLabelForms > 0 {
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "missing_form_labels",
				"severity":    "high",
				"description": "Form fields are missing proper labels",
				"count":       result.MissingLabelForms,
			})
			result.Recommendations = append(result.Recommendations, "Add explicit labels to all form fields")
		}

		// Check for ARIA attributes
		result.AriaAttributesUsed = doc.Find("[aria-*]").Length() > 0
		if !result.AriaAttributesUsed {
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "no_aria",
				"severity":    "medium",
				"description": "No ARIA attributes found for assistive technologies",
			})
			result.Recommendations = append(result.Recommendations, "Use ARIA attributes to improve accessibility for assistive technologies")
		}

		// Check for semantic HTML
		semanticTags := []string{"header", "footer", "nav", "main", "article", "section", "aside"}
		semanticFound := 0
		for _, tag := range semanticTags {
			count := doc.Find(tag).Length()
			if count > 0 {
				semanticFound++
			}
		}
		result.SemanticHTMLUsed = semanticFound >= 3 // Require at least 3 different semantic elements

		if !result.SemanticHTMLUsed {
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "insufficient_semantic_html",
				"severity":    "medium",
				"description": "Insufficient use of semantic HTML elements",
			})
			result.Recommendations = append(result.Recommendations, "Use more semantic HTML elements (header, nav, main, etc.)")
		}

		// Check for skip links for keyboard navigation
		result.HasSkipLinks = doc.Find("a[href='#content'], a[href='#main']").Length() > 0
		if !result.HasSkipLinks {
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "no_skip_links",
				"severity":    "medium",
				"description": "No skip links found for keyboard navigation",
			})
			result.Recommendations = append(result.Recommendations, "Add skip links to bypass navigation for keyboard users")
		}

		// Check for tabindex issues
		doc.Find("[tabindex]").Each(func(i int, s *goquery.Selection) {
			tabindex, exists := s.Attr("tabindex")
			if exists && tabindex != "0" && tabindex != "-1" {
				result.TabindexIssues = append(result.TabindexIssues, s.Text())
				result.Issues = append(result.Issues, map[string]interface{}{
					"type":        "tabindex_issue",
					"severity":    "medium",
					"description": "Avoid using tabindex values greater than 0",
					"element":     s.Text(),
				})
			}
		})
		if len(result.TabindexIssues) > 0 {
			result.Recommendations = append(result.Recommendations, "Use tabindex values of only 0 or -1")
		}

		// Check for accessible forms (required attributes)
		formWithRequired := doc.Find("form input[required], form [aria-required='true']").Length()
		result.AccessibleForms = formWithRequired > 0 || doc.Find("form").Length() == 0

		if !result.AccessibleForms && doc.Find("form").Length() > 0 {
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "inaccessible_forms",
				"severity":    "medium",
				"description": "Forms should indicate required fields",
			})
			result.Recommendations = append(result.Recommendations, "Mark required form fields with the required attribute or aria-required")
		}
	}

	// Simple check for potential contrast issues (just a heuristic based on color keywords in CSS)
	lowContrastColors := []string{"white", "#fff", "#ffffff", "yellow", "#ffff00", "lightgray", "lightgrey", "#d3d3d3"}
	for _, color := range lowContrastColors {
		if strings.Contains(data.HTML, "color:"+color) || strings.Contains(data.HTML, "color: "+color) {
			result.ContrastIssues = append(result.ContrastIssues, map[string]interface{}{
				"type":  "potential_low_contrast",
				"color": color,
			})
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "potential_contrast_issue",
				"severity":    "medium",
				"description": "Potential low contrast text detected",
				"color":       color,
			})
		}
	}
	if len(result.ContrastIssues) > 0 {
		result.Recommendations = append(result.Recommendations, "Ensure sufficient color contrast for all text")
	}

	// Check for font size issues
	smallFontRegex := regexp.MustCompile(`font-size: ?([0-9]{1,2})(px|pt);`)
	matches := smallFontRegex.FindAllStringSubmatch(data.HTML, -1)
	hasSmallFont := false
	for _, match := range matches {
		if len(match) >= 3 {
			if match[2] == "px" && match[1] < "12" {
				hasSmallFont = true
				break
			} else if match[2] == "pt" && match[1] < "9" {
				hasSmallFont = true
				break
			}
		}
	}
	if hasSmallFont {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "small_font_size",
			"severity":    "medium",
			"description": "Font size may be too small for readability",
		})
		result.Recommendations = append(result.Recommendations, "Use font sizes of at least 12px or 9pt for readability")
	}

	// Calculate score based on issues
	score := 100.0
	for _, issue := range result.Issues {
		severity := issue["severity"].(string)
		switch severity {
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	result.Score = score

	return map[string]interface{}{
		"score":                result.Score,
		"missing_alt_text":     result.MissingAltText,
		"missing_label_forms":  result.MissingLabelForms,
		"contrast_issues":      result.ContrastIssues,
		"aria_attributes_used": result.AriaAttributesUsed,
		"semantic_html_used":   result.SemanticHTMLUsed,
		"has_skip_links":       result.HasSkipLinks,
		"tabindex_issues":      result.TabindexIssues,
		"accessible_forms":     result.AccessibleForms,
		"issues":               result.Issues,
		"recommendations":      result.Recommendations,
	}
}
