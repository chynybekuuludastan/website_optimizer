package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// LighthouseAnalyzer analyzes a website using Google Lighthouse
type LighthouseAnalyzer struct {
	*BaseAnalyzer
	LighthouseClient *lighthouse.Client
	Config           *config.Config
}

// NewLighthouseAnalyzer creates a new Lighthouse analyzer
func NewLighthouseAnalyzer(cfg *config.Config) *LighthouseAnalyzer {
	return &LighthouseAnalyzer{
		BaseAnalyzer:     NewBaseAnalyzer(LighthouseType),
		LighthouseClient: lighthouse.NewClient(cfg.LighthouseURL, cfg.LighthouseAPIKey),
		Config:           cfg,
	}
}

// Analyze performs a website analysis using Lighthouse
func (a *LighthouseAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set up audit options
	options := lighthouse.DefaultAuditOptions()

	// Configure all analysis categories
	options.Categories = []lighthouse.Category{
		lighthouse.CategoryPerformance,   // Performance
		lighthouse.CategoryAccessibility, // Accessibility
		lighthouse.CategoryBestPractices, // Best practices
		lighthouse.CategorySEO,           // SEO
	}

	// Set mobile or desktop mode
	if a.Config.LighthouseMobileMode {
		options.FormFactor = lighthouse.FormFactorMobile
	} else {
		options.FormFactor = lighthouse.FormFactorDesktop
	}

	// Set locale
	options.Locale = "ru"

	// Log analysis start
	startTime := time.Now()
	a.SetMetric("lighthouse_start_time", startTime.Format(time.RFC3339))
	a.SetMetric("lighthouse_url", data.URL)

	// Run full analysis and convert results
	result, err := a.LighthouseClient.AnalyzeURL(ctx, data.URL, options)
	if err != nil {
		a.AddIssue(map[string]interface{}{
			"type":        "lighthouse_error",
			"severity":    "high",
			"description": "Error running Lighthouse audit",
			"error":       err.Error(),
		})
		// Set minimal metrics for error reporting
		a.SetMetric("lighthouse_error", err.Error())
		a.SetMetric("lighthouse_success", false)
		return a.GetMetrics(), fmt.Errorf("lighthouse analysis error: %w", err)
	}

	// Analysis succeeded
	a.SetMetric("lighthouse_success", true)
	a.SetMetric("lighthouse_end_time", time.Now().Format(time.RFC3339))
	a.SetMetric("lighthouse_version", result.LighthouseVersion)
	a.SetMetric("lighthouse_fetch_time", result.FetchTime)
	a.SetMetric("lighthouse_total_time", result.TotalAnalysisTime)
	a.SetMetric("analysis_duration", time.Since(startTime).Seconds())

	// Add performance metrics
	a.SetMetric("performance_metrics", result.Metrics)

	// Add category scores
	categoryScores := make(map[string]float64)
	for category, score := range result.Scores {
		categoryScores[category] = score
	}
	a.SetMetric("category_scores", categoryScores)

	// Add important audits
	// Save full audit data for use by other analyzers
	a.SetMetric("audits", result.Audits)

	// Add issues from Lighthouse
	for _, issue := range result.Issues {
		a.AddIssue(issue)
	}

	// Add recommendations from Lighthouse
	for _, recommendation := range result.Recommendations {
		a.AddRecommendation(recommendation)
	}

	// Calculate overall score based on categories
	totalScore := 0.0
	count := 0

	for _, score := range result.Scores {
		totalScore += score
		count++
	}

	score := 0.0
	if count > 0 {
		score = totalScore / float64(count) * 100 // Normalize to 100
	}

	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

func (a *LighthouseAnalyzer) GetPerformanceMetricsString() string {
	metrics, ok := a.GetMetrics()["performance_metrics"]
	if !ok {
		return "Performance metrics not available"
	}

	metricsMap, ok := metrics.(lighthouse.MetricsResult)
	if !ok {
		return "Invalid performance metrics format"
	}

	result := "Performance metrics:\n"
	result += fmt.Sprintf("- First Contentful Paint: %.1f ms\n", metricsMap.FirstContentfulPaint)
	result += fmt.Sprintf("- Largest Contentful Paint: %.1f ms\n", metricsMap.LargestContentfulPaint)
	result += fmt.Sprintf("- Total Blocking Time: %.1f ms\n", metricsMap.TotalBlockingTime)
	result += fmt.Sprintf("- Cumulative Layout Shift: %.3f\n", metricsMap.CumulativeLayoutShift)
	result += fmt.Sprintf("- Time to Interactive: %.1f ms\n", metricsMap.TimeToInteractive)
	result += fmt.Sprintf("- Speed Index: %.1f ms\n", metricsMap.SpeedIndex)

	return result
}
