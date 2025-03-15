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
		BaseAnalyzer:     NewBaseAnalyzer(),
		LighthouseClient: lighthouse.NewClient(cfg.LighthouseURL, "AIzaSyDtb0Y0XFZrWE1GN3GkKKnN00pzHWnK-Dk"),
		Config:           cfg,
	}
}

// Analyze выполняет анализ сайта с помощью Lighthouse
func (a *LighthouseAnalyzer) Analyze(data *parser.WebsiteData) (map[string]interface{}, error) {
	// Настройка опций аудита с учетом предпочитаемого устройства
	options := lighthouse.DefaultAuditOptions()

	// Определяем все нужные категории анализа
	options.Categories = []lighthouse.Category{
		lighthouse.CategoryPerformance,   // Производительность
		lighthouse.CategoryAccessibility, // Доступность
		lighthouse.CategoryBestPractices, // Лучшие практики
		lighthouse.CategorySEO,           // SEO
	}

	// Устанавливаем мобильный или десктопный режим на основе конфигурации
	if a.Config.LighthouseMobileMode {
		options.FormFactor = lighthouse.FormFactorMobile
	} else {
		options.FormFactor = lighthouse.FormFactorDesktop
	}

	// Устанавливаем локаль
	options.Locale = "ru"

	// Выполнение аудита с таймаутом
	// Добавляем дополнительный запас времени к таймауту на запрос
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(a.Config.AnalysisTimeout+30)*time.Second,
	)
	defer cancel()

	// Логируем начало анализа
	a.SetMetric("lighthouse_start_time", time.Now().Format(time.RFC3339))
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

	// Добавляем метрики производительности
	a.SetMetric("performance_metrics", result.Metrics)

	// Добавляем оценки по категориям
	categoryScores := make(map[string]float64)
	for category, score := range result.Scores {
		categoryScores[category] = score
	}
	a.SetMetric("category_scores", categoryScores)

	// Добавляем важные аудиты
	importantAudits := make(map[string]interface{})
	for id, audit := range result.Audits {
		if audit.Score < 0.9 {
			importantAudits[id] = map[string]interface{}{
				"title":         audit.Title,
				"description":   audit.Description,
				"score":         audit.Score,
				"display_value": audit.DisplayValue,
			}
		}
	}
	a.SetMetric("important_audits", importantAudits)

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
		score = totalScore / float64(count)
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
