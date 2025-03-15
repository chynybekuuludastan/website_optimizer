package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// LighthouseAnalyzer анализирует сайт с помощью Lighthouse
type LighthouseAnalyzer struct {
	*BaseAnalyzer
	LighthouseClient *lighthouse.Client
	Config           *config.Config
}

// NewLighthouseAnalyzer создает новый анализатор Lighthouse
func NewLighthouseAnalyzer(cfg *config.Config) *LighthouseAnalyzer {
	return &LighthouseAnalyzer{
		BaseAnalyzer:     NewBaseAnalyzer(LighthouseType),
		LighthouseClient: lighthouse.NewClient(cfg.LighthouseURL, cfg.LighthouseAPIKey),
		Config:           cfg,
	}
}

// Analyze выполняет анализ сайта с помощью Lighthouse
func (a *LighthouseAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Настройка опций аудита
	options := lighthouse.DefaultAuditOptions()

	// Определяем все нужные категории анализа
	options.Categories = []lighthouse.Category{
		lighthouse.CategoryPerformance,   // Производительность
		lighthouse.CategoryAccessibility, // Доступность
		lighthouse.CategoryBestPractices, // Лучшие практики
		lighthouse.CategorySEO,           // SEO
	}

	// Устанавливаем мобильный или десктопный режим
	if a.Config.LighthouseMobileMode {
		options.FormFactor = lighthouse.FormFactorMobile
	} else {
		options.FormFactor = lighthouse.FormFactorDesktop
	}

	// Устанавливаем локаль
	options.Locale = "ru"

	// Логируем начало анализа
	startTime := time.Now()
	a.SetMetric("lighthouse_start_time", startTime.Format(time.RFC3339))
	a.SetMetric("lighthouse_url", data.URL)

	// Запускаем полный анализ с преобразованием результатов
	result, err := a.LighthouseClient.AnalyzeURL(ctx, data.URL, options)
	if err != nil {
		a.AddIssue(map[string]interface{}{
			"type":        "lighthouse_error",
			"severity":    "high",
			"description": "Ошибка при выполнении аудита Lighthouse",
			"error":       err.Error(),
		})
		// Устанавливаем минимальные метрики для отчета об ошибке
		a.SetMetric("lighthouse_error", err.Error())
		a.SetMetric("lighthouse_success", false)
		return a.GetMetrics(), fmt.Errorf("ошибка Lighthouse анализа: %w", err)
	}

	// Анализ прошел успешно
	a.SetMetric("lighthouse_success", true)
	a.SetMetric("lighthouse_end_time", time.Now().Format(time.RFC3339))
	a.SetMetric("lighthouse_version", result.LighthouseVersion)
	a.SetMetric("lighthouse_fetch_time", result.FetchTime)
	a.SetMetric("lighthouse_total_time", result.TotalAnalysisTime)
	a.SetMetric("analysis_duration", time.Since(startTime).Seconds())

	// Добавляем метрики производительности
	a.SetMetric("performance_metrics", result.Metrics)

	// Добавляем оценки по категориям
	categoryScores := make(map[string]float64)
	for category, score := range result.Scores {
		categoryScores[category] = score
	}
	a.SetMetric("category_scores", categoryScores)

	// Добавляем важные аудиты
	// Сохраняем полные данные аудитов для использования другими анализаторами
	a.SetMetric("audits", result.Audits)

	// Добавляем проблемы из Lighthouse
	for _, issue := range result.Issues {
		a.AddIssue(issue)
	}

	// Добавляем рекомендации из Lighthouse
	for _, recommendation := range result.Recommendations {
		a.AddRecommendation(recommendation)
	}

	// Расчет общей оценки на основе категорий
	totalScore := 0.0
	count := 0

	for _, score := range result.Scores {
		totalScore += score
		count++
	}

	score := 0.0
	if count > 0 {
		score = totalScore / float64(count) * 100 // Нормализуем до 100
	}

	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// GetPerformanceMetricsString возвращает строковое представление метрик производительности
func (a *LighthouseAnalyzer) GetPerformanceMetricsString() string {
	metrics, ok := a.GetMetrics()["performance_metrics"]
	if !ok {
		return "Метрики производительности недоступны"
	}

	metricsMap, ok := metrics.(lighthouse.MetricsResult)
	if !ok {
		return "Некорректный формат метрик производительности"
	}

	result := "Метрики производительности:\n"
	result += fmt.Sprintf("- First Contentful Paint: %.1f ms\n", metricsMap.FirstContentfulPaint)
	result += fmt.Sprintf("- Largest Contentful Paint: %.1f ms\n", metricsMap.LargestContentfulPaint)
	result += fmt.Sprintf("- Total Blocking Time: %.1f ms\n", metricsMap.TotalBlockingTime)
	result += fmt.Sprintf("- Cumulative Layout Shift: %.3f\n", metricsMap.CumulativeLayoutShift)
	result += fmt.Sprintf("- Time to Interactive: %.1f ms\n", metricsMap.TimeToInteractive)
	result += fmt.Sprintf("- Speed Index: %.1f ms\n", metricsMap.SpeedIndex)

	return result
}
