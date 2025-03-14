package analyzer

import (
	"regexp"
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// PerformanceResult contains the performance analysis results
type PerformanceResult struct {
	Score                float64                  `json:"score"`
	LoadTime             float64                  `json:"load_time_seconds"`
	TotalPageSize        int64                    `json:"total_page_size_bytes"`
	NumRequests          int                      `json:"num_requests"`
	LargeImages          []map[string]interface{} `json:"large_images"`
	UnminifiedCSS        []string                 `json:"unminified_css"`
	UnminifiedJS         []string                 `json:"unminified_js"`
	RenderBlockingAssets []string                 `json:"render_blocking_assets"`
	Issues               []map[string]interface{} `json:"issues"`
	Recommendations      []string                 `json:"recommendations"`
}

// AnalyzePerformance performs performance analysis on the website data
func AnalyzePerformance(data *parser.WebsiteData) map[string]interface{} {
	result := PerformanceResult{
		LoadTime:             float64(data.LoadTime.Milliseconds()) / 1000.0,
		LargeImages:          []map[string]interface{}{},
		UnminifiedCSS:        []string{},
		UnminifiedJS:         []string{},
		RenderBlockingAssets: []string{},
		Issues:               []map[string]interface{}{},
		Recommendations:      []string{},
	}

	// Calculate total page size (simplified estimate)
	totalSize := int64(len(data.HTML))
	result.TotalPageSize = totalSize

	// Count total requests
	result.NumRequests = len(data.Images) + len(data.Scripts) + len(data.Styles) + 1 // +1 for main HTML

	// Check for large images
	for _, img := range data.Images {
		if img.FileSize > 100000 { // 100KB threshold
			result.LargeImages = append(result.LargeImages, map[string]interface{}{
				"url":  img.URL,
				"size": img.FileSize,
			})
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "large_image",
				"severity":    "medium",
				"description": "Image file is too large",
				"url":         img.URL,
				"size":        img.FileSize,
			})
		}
	}
	if len(result.LargeImages) > 0 {
		result.Recommendations = append(result.Recommendations, "Optimize large images to improve page load time")
	}

	// Check for render-blocking resources
	for _, script := range data.Scripts {
		// Check if script is in the head and doesn't have async/defer
		if !strings.Contains(script.URL, "async") && !strings.Contains(script.URL, "defer") {
			result.RenderBlockingAssets = append(result.RenderBlockingAssets, script.URL)
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "render_blocking_script",
				"severity":    "medium",
				"description": "Script may be render-blocking",
				"url":         script.URL,
			})
		}
	}
	if len(result.RenderBlockingAssets) > 0 {
		result.Recommendations = append(result.Recommendations, "Add async or defer attributes to render-blocking scripts")
	}

	// Simple check for unminified CSS (based on whitespace)
	for _, style := range data.Styles {
		if strings.Contains(style.URL, ".min.css") {
			continue // Already minified
		}
		result.UnminifiedCSS = append(result.UnminifiedCSS, style.URL)
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "unminified_css",
			"severity":    "low",
			"description": "CSS file might not be minified",
			"url":         style.URL,
		})
	}
	if len(result.UnminifiedCSS) > 0 {
		result.Recommendations = append(result.Recommendations, "Minify CSS files to reduce size")
	}

	// Simple check for unminified JS (based on filename)
	for _, script := range data.Scripts {
		if strings.Contains(script.URL, ".min.js") {
			continue // Already minified
		}
		result.UnminifiedJS = append(result.UnminifiedJS, script.URL)
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "unminified_js",
			"severity":    "low",
			"description": "JavaScript file might not be minified",
			"url":         script.URL,
		})
	}
	if len(result.UnminifiedJS) > 0 {
		result.Recommendations = append(result.Recommendations, "Minify JavaScript files to reduce size")
	}

	// Check HTML for inline styles (which can be render-blocking)
	inlineStyleRegex := regexp.MustCompile(`<style\b[^>]*>(.*?)</style>`)
	if inlineStyleRegex.MatchString(data.HTML) {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "inline_css",
			"severity":    "low",
			"description": "Page contains inline CSS which may block rendering",
		})
		result.Recommendations = append(result.Recommendations, "Move inline CSS to external stylesheets")
	}

	// Load time evaluations
	if result.LoadTime > 3.0 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "slow_load_time",
			"severity":    "high",
			"description": "Page load time is too slow",
			"load_time":   result.LoadTime,
			"threshold":   3.0,
		})
		result.Recommendations = append(result.Recommendations, "Improve page load time to enhance user experience")
	}

	// Page size evaluations
	if result.TotalPageSize > 2*1024*1024 { // 2MB threshold
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "large_page_size",
			"severity":    "medium",
			"description": "Total page size is too large",
			"size":        result.TotalPageSize,
			"threshold":   2 * 1024 * 1024,
		})
		result.Recommendations = append(result.Recommendations, "Reduce total page size to improve load time")
	}

	// Number of requests evaluations
	if result.NumRequests > 50 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "too_many_requests",
			"severity":    "medium",
			"description": "Page makes too many HTTP requests",
			"count":       result.NumRequests,
			"threshold":   50,
		})
		result.Recommendations = append(result.Recommendations, "Reduce the number of HTTP requests by combining resources")
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

	// Adjust score based on load time
	if result.LoadTime > 0 {
		if result.LoadTime < 1.0 {
			score += 10
		} else if result.LoadTime > 5.0 {
			score -= 20
		} else if result.LoadTime > 3.0 {
			score -= 10
		}
	}

	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}
	result.Score = score

	return map[string]interface{}{
		"score":                  result.Score,
		"load_time_seconds":      result.LoadTime,
		"total_page_size_bytes":  result.TotalPageSize,
		"num_requests":           result.NumRequests,
		"large_images":           result.LargeImages,
		"unminified_css":         result.UnminifiedCSS,
		"unminified_js":          result.UnminifiedJS,
		"render_blocking_assets": result.RenderBlockingAssets,
		"issues":                 result.Issues,
		"recommendations":        result.Recommendations,
	}
}
