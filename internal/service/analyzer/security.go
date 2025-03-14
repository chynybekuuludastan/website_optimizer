package analyzer

import (
	"net/url"
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// SecurityResult contains the security analysis results
type SecurityResult struct {
	Score             float64                  `json:"score"`
	HasHTTPS          bool                     `json:"has_https"`
	MissingSecHeaders []string                 `json:"missing_security_headers"`
	HasCSP            bool                     `json:"has_content_security_policy"`
	HasXSSProtection  bool                     `json:"has_xss_protection"`
	HasHSTS           bool                     `json:"has_hsts"`
	InsecureCookies   []string                 `json:"insecure_cookies"`
	MixedContent      []string                 `json:"mixed_content"`
	Issues            []map[string]interface{} `json:"issues"`
	Recommendations   []string                 `json:"recommendations"`
}

// AnalyzeSecurity performs security analysis on the website data
func AnalyzeSecurity(data *parser.WebsiteData) map[string]interface{} {
	result := SecurityResult{
		MissingSecHeaders: []string{},
		InsecureCookies:   []string{},
		MixedContent:      []string{},
		Issues:            []map[string]interface{}{},
		Recommendations:   []string{},
	}

	// Check for HTTPS
	parsedURL, err := url.Parse(data.URL)
	if err == nil {
		result.HasHTTPS = parsedURL.Scheme == "https"
	}

	if !result.HasHTTPS {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "no_https",
			"severity":    "high",
			"description": "The website does not use HTTPS",
		})
		result.Recommendations = append(result.Recommendations, "Enable HTTPS to secure data transmission")
	}

	// Check for security headers
	// Note: In a real implementation, these would come from HTTP response headers
	// For this example, we'll just detect them from HTML meta tags or assume they're missing

	// Check for Content-Security-Policy
	result.HasCSP = false
	for key, value := range data.MetaTags {
		if key == "content-security-policy" && value != "" {
			result.HasCSP = true
			break
		}
	}
	if !result.HasCSP {
		result.MissingSecHeaders = append(result.MissingSecHeaders, "Content-Security-Policy")
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_csp",
			"severity":    "medium",
			"description": "Content Security Policy is not implemented",
		})
		result.Recommendations = append(result.Recommendations, "Implement Content Security Policy to prevent XSS attacks")
	}

	// Check for X-XSS-Protection
	result.HasXSSProtection = false
	for key, value := range data.MetaTags {
		if key == "x-xss-protection" && value != "" {
			result.HasXSSProtection = true
			break
		}
	}
	if !result.HasXSSProtection {
		result.MissingSecHeaders = append(result.MissingSecHeaders, "X-XSS-Protection")
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_xss_protection",
			"severity":    "medium",
			"description": "X-XSS-Protection header is not implemented",
		})
		result.Recommendations = append(result.Recommendations, "Add X-XSS-Protection header to help prevent XSS attacks")
	}

	// Check for HSTS
	result.HasHSTS = false
	for key, value := range data.MetaTags {
		if key == "strict-transport-security" && value != "" {
			result.HasHSTS = true
			break
		}
	}
	if !result.HasHSTS && result.HasHTTPS {
		result.MissingSecHeaders = append(result.MissingSecHeaders, "Strict-Transport-Security")
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_hsts",
			"severity":    "medium",
			"description": "HTTP Strict Transport Security is not implemented",
		})
		result.Recommendations = append(result.Recommendations, "Implement HSTS to enforce secure connections")
	}

	// Check for mixed content
	if result.HasHTTPS {
		// Check for HTTP resources on HTTPS page
		for _, img := range data.Images {
			if strings.HasPrefix(img.URL, "http:") {
				result.MixedContent = append(result.MixedContent, img.URL)
				result.Issues = append(result.Issues, map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Mixed content: HTTP resource on HTTPS page",
					"url":         img.URL,
				})
			}
		}
		for _, script := range data.Scripts {
			if strings.HasPrefix(script.URL, "http:") {
				result.MixedContent = append(result.MixedContent, script.URL)
				result.Issues = append(result.Issues, map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Mixed content: HTTP script on HTTPS page",
					"url":         script.URL,
				})
			}
		}
		for _, style := range data.Styles {
			if strings.HasPrefix(style.URL, "http:") {
				result.MixedContent = append(result.MixedContent, style.URL)
				result.Issues = append(result.Issues, map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Mixed content: HTTP stylesheet on HTTPS page",
					"url":         style.URL,
				})
			}
		}
		if len(result.MixedContent) > 0 {
			result.Recommendations = append(result.Recommendations, "Fix mixed content by updating all resources to use HTTPS")
		}
	}

	// Check for forms without CSRF protection
	if strings.Contains(data.HTML, "<form") && !strings.Contains(data.HTML, "csrf") {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "possible_csrf_vulnerability",
			"severity":    "high",
			"description": "Forms found without obvious CSRF protection",
		})
		result.Recommendations = append(result.Recommendations, "Implement CSRF tokens for all forms")
	}

	// Check for use of inline JavaScript (potential XSS vulnerability)
	if strings.Contains(data.HTML, "<script>") || strings.Contains(data.HTML, "javascript:") {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "inline_js",
			"severity":    "medium",
			"description": "Inline JavaScript found, which can be a security risk",
		})
		result.Recommendations = append(result.Recommendations, "Avoid inline JavaScript and use external scripts with CSP")
	}

	// Calculate score based on issues
	score := 100.0
	for _, issue := range result.Issues {
		severity := issue["severity"].(string)
		switch severity {
		case "high":
			score -= 20
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// HTTPS has a big impact on score
	if !result.HasHTTPS {
		score -= 30
	}

	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	result.Score = score

	return map[string]interface{}{
		"score":                       result.Score,
		"has_https":                   result.HasHTTPS,
		"missing_security_headers":    result.MissingSecHeaders,
		"has_content_security_policy": result.HasCSP,
		"has_xss_protection":          result.HasXSSProtection,
		"has_hsts":                    result.HasHSTS,
		"mixed_content":               result.MixedContent,
		"issues":                      result.Issues,
		"recommendations":             result.Recommendations,
	}
}
