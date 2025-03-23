package analyzer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// Пороговые значения для Core Web Vitals (по документации Google)
const (
	// Largest Contentful Paint (ms)
	LCPGood             = 2500.0
	LCPNeedsImprovement = 4000.0

	// Cumulative Layout Shift
	CLSGood             = 0.1
	CLSNeedsImprovement = 0.25

	// First Input Delay (ms) или Total Blocking Time как его аналог
	TBTGood             = 200.0
	TBTNeedsImprovement = 600.0
)

// LighthouseAnalyzer анализирует веб-сайт с помощью Google Lighthouse
type LighthouseAnalyzer struct {
	*BaseAnalyzer
	LighthouseClient *lighthouse.Client
	Config           *config.Config
	// Флаги для дополнительных настроек анализа
	DisableCaching  bool
	RetryCount      int
	AnalysisTimeout time.Duration
}

// NewLighthouseAnalyzer создает новый Lighthouse анализатор
func NewLighthouseAnalyzer(cfg *config.Config) *LighthouseAnalyzer {
	// Создаем Lighthouse клиент с дополнительными опциями
	client := lighthouse.NewClient(
		cfg.LighthouseURL,
		cfg.LighthouseAPIKey,
		lighthouse.WithRetries(3),
		lighthouse.WithCacheTTL(6*time.Hour), // Кешируем результаты на 6 часов
		lighthouse.WithRateLimit(2.0),        // Ограничиваем количество запросов
	)

	return &LighthouseAnalyzer{
		BaseAnalyzer:     NewBaseAnalyzer(LighthouseType),
		LighthouseClient: client,
		Config:           cfg,
		RetryCount:       3,
		AnalysisTimeout:  60 * time.Second,
	}
}

// Analyze выполняет анализ веб-сайта с помощью Lighthouse
func (a *LighthouseAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Создаем контекст с таймаутом для ограничения времени выполнения
	analysisCtx, cancel := context.WithTimeout(ctx, a.AnalysisTimeout)
	defer cancel()

	// Проверяем контекст перед выполнением
	select {
	case <-analysisCtx.Done():
		return nil, fmt.Errorf("анализ отменен до начала выполнения: %w", analysisCtx.Err())
	default:
	}

	// Устанавливаем параметры аудита
	options := lighthouse.DefaultAuditOptions()

	// Настраиваем все категории анализа
	options.Categories = []lighthouse.Category{
		lighthouse.CategoryPerformance,   // Производительность
		lighthouse.CategoryAccessibility, // Доступность
		lighthouse.CategoryBestPractices, // Лучшие практики
		lighthouse.CategorySEO,           // SEO
	}

	// Устанавливаем мобильный или десктопный режим в зависимости от конфигурации
	if a.Config.LighthouseMobileMode {
		options.FormFactor = lighthouse.FormFactorMobile
	} else {
		options.FormFactor = lighthouse.FormFactorDesktop
	}

	// Устанавливаем локаль
	options.Locale = "ru"

	// Настраиваем кеширование
	if a.DisableCaching {
		options.CacheTTL = 0
	} else {
		// Устанавливаем уникальный reference time для предотвращения проблем с кешем
		options.ReferenceTime = time.Now()
	}

	// Логируем начало анализа
	startTime := time.Now()
	log.Printf("Начинаем анализ Lighthouse для %s, форм-фактор: %s", data.URL, options.FormFactor)
	a.SetMetric("lighthouse_start_time", startTime.Format(time.RFC3339))
	a.SetMetric("lighthouse_url", data.URL)

	// Выполняем полный анализ и обрабатываем результаты
	result, err := a.LighthouseClient.AnalyzeURL(analysisCtx, data.URL, options)
	if err != nil {
		// Обрабатываем ошибку и добавляем информацию о ней в метрики
		a.AddIssue(map[string]interface{}{
			"type":        "lighthouse_error",
			"severity":    "high",
			"description": "Ошибка при выполнении Lighthouse аудита",
			"error":       err.Error(),
		})

		// Устанавливаем метрики для отчетности об ошибке
		a.SetMetric("lighthouse_error", err.Error())
		a.SetMetric("lighthouse_success", false)
		a.SetMetric("lighthouse_error_time", time.Now().Format(time.RFC3339))
		a.SetMetric("analysis_duration_ms", time.Since(startTime).Milliseconds())

		return a.GetMetrics(), fmt.Errorf("ошибка анализа Lighthouse: %w", err)
	}

	// Анализ успешно выполнен
	a.SetMetric("lighthouse_success", true)
	a.SetMetric("lighthouse_end_time", time.Now().Format(time.RFC3339))
	a.SetMetric("lighthouse_version", result.LighthouseVersion)
	a.SetMetric("lighthouse_fetch_time", result.FetchTime)
	a.SetMetric("lighthouse_total_time", result.TotalAnalysisTime)
	a.SetMetric("analysis_duration_ms", time.Since(startTime).Milliseconds())

	// Обрабатываем метрики производительности, включая Core Web Vitals
	a.processPerformanceMetrics(result.Metrics)

	// Добавляем результаты по категориям
	categoryScores := make(map[string]float64)
	for category, score := range result.Scores {
		categoryScores[category] = score

		// Добавляем также отдельные метрики для каждой категории
		a.SetMetric(fmt.Sprintf("%s_score", category), score)

		// Добавляем рекомендации на основе скора категории
		a.addCategoryRecommendation(category, score)
	}
	a.SetMetric("category_scores", categoryScores)

	// Сохраняем полные данные аудита для использования другими анализаторами
	a.SetMetric("audits", result.Audits)

	// Обрабатываем аудиты по категориям
	a.processAudits(result.Audits)

	// Добавляем проблемы из Lighthouse
	for _, issue := range result.Issues {
		a.AddIssue(issue)
	}

	// Добавляем рекомендации из Lighthouse
	for _, recommendation := range result.Recommendations {
		a.AddRecommendation(recommendation)
	}

	// Рассчитываем общий балл на основе категорий
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

	// Добавляем итоговую информацию
	a.SetMetric("completion_time", time.Now().Format(time.RFC3339))
	a.SetMetric("total_issues_count", len(a.GetIssues()))
	a.SetMetric("total_recommendations_count", len(a.GetRecommendations()))

	return a.GetMetrics(), nil
}

// processPerformanceMetrics обрабатывает метрики производительности, включая Core Web Vitals
func (a *LighthouseAnalyzer) processPerformanceMetrics(metrics lighthouse.MetricsResult) {
	// Сохраняем все метрики
	a.SetMetric("performance_metrics", metrics)

	// Добавляем отдельные метрики для более удобного доступа
	a.SetMetric("first_contentful_paint", metrics.FirstContentfulPaint)
	a.SetMetric("largest_contentful_paint", metrics.LargestContentfulPaint)
	a.SetMetric("first_meaningful_paint", metrics.FirstMeaningfulPaint)
	a.SetMetric("speed_index", metrics.SpeedIndex)
	a.SetMetric("time_to_interactive", metrics.TimeToInteractive)
	a.SetMetric("total_blocking_time", metrics.TotalBlockingTime)
	a.SetMetric("cumulative_layout_shift", metrics.CumulativeLayoutShift)

	// Обрабатываем Core Web Vitals (LCP, CLS, TBT как прокси для FID)

	// Largest Contentful Paint (LCP)
	lcp := metrics.LargestContentfulPaint
	var lcpSeverity string
	var lcpMessage string

	if lcp <= LCPGood {
		lcpSeverity = "low"
		lcpMessage = "LCP в пределах нормы"
	} else if lcp <= LCPNeedsImprovement {
		lcpSeverity = "medium"
		lcpMessage = "LCP требует улучшения"
		a.AddRecommendation("Оптимизируйте Largest Contentful Paint (LCP) для улучшения производительности загрузки основного контента страницы.")
	} else {
		lcpSeverity = "high"
		lcpMessage = "LCP значительно превышает рекомендуемые значения"
		a.AddRecommendation("Срочно оптимизируйте Largest Contentful Paint (LCP), который значительно превышает рекомендуемые значения.")
	}

	a.SetMetric("lcp_status", lcpSeverity)

	if lcpSeverity != "low" {
		a.AddIssue(map[string]interface{}{
			"type":        "core_web_vital_lcp",
			"severity":    lcpSeverity,
			"description": lcpMessage,
			"value":       lcp,
			"threshold":   LCPGood,
		})
	}

	// Cumulative Layout Shift (CLS)
	cls := metrics.CumulativeLayoutShift
	var clsSeverity string
	var clsMessage string

	if cls <= CLSGood {
		clsSeverity = "low"
		clsMessage = "CLS в пределах нормы"
	} else if cls <= CLSNeedsImprovement {
		clsSeverity = "medium"
		clsMessage = "CLS требует улучшения"
		a.AddRecommendation("Уменьшите Cumulative Layout Shift (CLS) для улучшения стабильности страницы при загрузке.")
	} else {
		clsSeverity = "high"
		clsMessage = "CLS значительно превышает рекомендуемые значения"
		a.AddRecommendation("Срочно исправьте проблемы со сдвигом макета (CLS), которые значительно влияют на пользовательский опыт.")
	}

	a.SetMetric("cls_status", clsSeverity)

	if clsSeverity != "low" {
		a.AddIssue(map[string]interface{}{
			"type":        "core_web_vital_cls",
			"severity":    clsSeverity,
			"description": clsMessage,
			"value":       cls,
			"threshold":   CLSGood,
		})
	}

	// Total Blocking Time (TBT как прокси для FID)
	tbt := metrics.TotalBlockingTime
	var tbtSeverity string
	var tbtMessage string

	if tbt <= TBTGood {
		tbtSeverity = "low"
		tbtMessage = "TBT в пределах нормы"
	} else if tbt <= TBTNeedsImprovement {
		tbtSeverity = "medium"
		tbtMessage = "TBT требует улучшения"
		a.AddRecommendation("Оптимизируйте Total Blocking Time (TBT) для улучшения интерактивности страницы.")
	} else {
		tbtSeverity = "high"
		tbtMessage = "TBT значительно превышает рекомендуемые значения"
		a.AddRecommendation("Срочно оптимизируйте JavaScript код для уменьшения Total Blocking Time (TBT) и улучшения интерактивности страницы.")
	}

	a.SetMetric("tbt_status", tbtSeverity)

	if tbtSeverity != "low" {
		a.AddIssue(map[string]interface{}{
			"type":        "core_web_vital_tbt",
			"severity":    tbtSeverity,
			"description": tbtMessage,
			"value":       tbt,
			"threshold":   TBTGood,
		})
	}

	// Добавляем общую метрику состояния Core Web Vitals
	if lcpSeverity == "high" || clsSeverity == "high" || tbtSeverity == "high" {
		a.SetMetric("core_web_vitals_status", "poor")
	} else if lcpSeverity == "medium" || clsSeverity == "medium" || tbtSeverity == "medium" {
		a.SetMetric("core_web_vitals_status", "needs_improvement")
	} else {
		a.SetMetric("core_web_vitals_status", "good")
	}
}

// processAudits обрабатывает результаты аудитов Lighthouse и добавляет проблемы и рекомендации
func (a *LighthouseAnalyzer) processAudits(audits map[string]lighthouse.Audit) {
	// Определяем отдельные группы аудитов по категориям для удобства обработки
	performanceAudits := make(map[string]bool)
	accessibilityAudits := make(map[string]bool)
	seoAudits := make(map[string]bool)
	bestPracticesAudits := make(map[string]bool)

	// Аудиты производительности
	performanceAudits["render-blocking-resources"] = true
	performanceAudits["uses-responsive-images"] = true
	performanceAudits["offscreen-images"] = true
	performanceAudits["unminified-css"] = true
	performanceAudits["unminified-javascript"] = true
	performanceAudits["unused-css-rules"] = true
	performanceAudits["unused-javascript"] = true
	performanceAudits["uses-optimized-images"] = true
	performanceAudits["uses-webp-images"] = true
	performanceAudits["uses-text-compression"] = true
	performanceAudits["uses-rel-preconnect"] = true
	performanceAudits["server-response-time"] = true
	performanceAudits["redirects"] = true
	performanceAudits["uses-rel-preload"] = true
	performanceAudits["efficient-animated-content"] = true
	performanceAudits["third-party-summary"] = true
	performanceAudits["third-party-facades"] = true
	performanceAudits["bootup-time"] = true
	performanceAudits["mainthread-work-breakdown"] = true
	performanceAudits["dom-size"] = true
	performanceAudits["resource-summary"] = true
	performanceAudits["font-display"] = true

	// Аудиты доступности
	accessibilityAudits["accesskeys"] = true
	accessibilityAudits["aria-allowed-attr"] = true
	accessibilityAudits["aria-hidden-body"] = true
	accessibilityAudits["aria-hidden-focus"] = true
	accessibilityAudits["aria-input-field-name"] = true
	accessibilityAudits["aria-required-attr"] = true
	accessibilityAudits["aria-required-children"] = true
	accessibilityAudits["aria-required-parent"] = true
	accessibilityAudits["aria-roles"] = true
	accessibilityAudits["aria-valid-attr"] = true
	accessibilityAudits["aria-valid-attr-value"] = true
	accessibilityAudits["button-name"] = true
	accessibilityAudits["bypass"] = true
	accessibilityAudits["color-contrast"] = true
	accessibilityAudits["definition-list"] = true
	accessibilityAudits["dlitem"] = true
	accessibilityAudits["document-title"] = true
	accessibilityAudits["duplicate-id-active"] = true
	accessibilityAudits["duplicate-id-aria"] = true
	accessibilityAudits["form-field-multiple-labels"] = true
	accessibilityAudits["frame-title"] = true
	accessibilityAudits["heading-order"] = true
	accessibilityAudits["html-has-lang"] = true
	accessibilityAudits["html-lang-valid"] = true
	accessibilityAudits["image-alt"] = true
	accessibilityAudits["input-image-alt"] = true
	accessibilityAudits["label"] = true
	accessibilityAudits["link-name"] = true
	accessibilityAudits["list"] = true
	accessibilityAudits["listitem"] = true
	accessibilityAudits["meta-refresh"] = true
	accessibilityAudits["meta-viewport"] = true
	accessibilityAudits["object-alt"] = true
	accessibilityAudits["tabindex"] = true
	accessibilityAudits["td-headers-attr"] = true
	accessibilityAudits["th-has-data-cells"] = true
	accessibilityAudits["valid-lang"] = true
	accessibilityAudits["video-caption"] = true

	// Аудиты SEO
	seoAudits["canonical"] = true
	seoAudits["font-size"] = true
	seoAudits["hreflang"] = true
	seoAudits["http-status-code"] = true
	seoAudits["is-crawlable"] = true
	seoAudits["link-text"] = true
	seoAudits["meta-description"] = true
	seoAudits["robots-txt"] = true
	seoAudits["structured-data"] = true
	seoAudits["tap-targets"] = true
	seoAudits["viewport"] = true

	// Аудиты по лучшим практикам
	bestPracticesAudits["appcache-manifest"] = true
	bestPracticesAudits["doctype"] = true
	bestPracticesAudits["charset"] = true
	bestPracticesAudits["dom-size"] = true
	bestPracticesAudits["external-anchors-use-rel-noopener"] = true
	bestPracticesAudits["geolocation-on-start"] = true
	bestPracticesAudits["no-document-write"] = true
	bestPracticesAudits["no-vulnerable-libraries"] = true
	bestPracticesAudits["js-libraries"] = true
	bestPracticesAudits["notification-on-start"] = true
	bestPracticesAudits["password-inputs-can-be-pasted-into"] = true
	bestPracticesAudits["uses-http2"] = true
	bestPracticesAudits["uses-passive-event-listeners"] = true
	bestPracticesAudits["meta-charset-utf-8"] = true
	bestPracticesAudits["no-unload-listeners"] = true
	bestPracticesAudits["deprecations"] = true
	bestPracticesAudits["errors-in-console"] = true
	bestPracticesAudits["image-aspect-ratio"] = true

	// Обрабатываем аудиты по категориям
	performanceIssues := extractLighthouseAuditsData(audits, performanceAudits)
	accessibilityIssues := extractLighthouseAuditsData(audits, accessibilityAudits)
	seoIssues := extractLighthouseAuditsData(audits, seoAudits)
	bestPracticesIssues := extractLighthouseAuditsData(audits, bestPracticesAudits)

	// Пороговое значение для определения проблем
	const minScoreThreshold = 0.9 // Аудиты с оценкой ниже 0.9 считаются проблемными

	// Добавляем проблемы из каждой категории
	for auditID, audit := range performanceIssues {
		a.addAuditIssue(auditID, audit, "performance", minScoreThreshold)
	}

	for auditID, audit := range accessibilityIssues {
		a.addAuditIssue(auditID, audit, "accessibility", minScoreThreshold)
	}

	for auditID, audit := range seoIssues {
		a.addAuditIssue(auditID, audit, "seo", minScoreThreshold)
	}

	for auditID, audit := range bestPracticesIssues {
		a.addAuditIssue(auditID, audit, "best_practices", minScoreThreshold)
	}
}

// addAuditIssue добавляет проблему на основе аудита, если оценка ниже порога
func (a *LighthouseAnalyzer) addAuditIssue(auditID string, audit lighthouse.Audit, category string, minScoreThreshold float64) {
	// Пропускаем информационные аудиты, которые не имеют оценки
	if audit.ScoreDisplayMode == "informative" || audit.ScoreDisplayMode == "manual" || audit.ScoreDisplayMode == "notApplicable" {
		return
	}

	// Если оценка ниже порога и не равна -1 (информационные аудиты часто имеют -1)
	if audit.Score < minScoreThreshold && audit.Score >= 0 {
		severity := getSeverityFromScore(audit.Score)

		// Создаем проблему
		a.AddIssue(map[string]interface{}{
			"type":        fmt.Sprintf("%s_%s", category, auditID),
			"severity":    severity,
			"description": audit.Title,
			"details":     audit.Description,
			"score":       audit.Score,
			"category":    category,
		})

		// Добавляем рекомендацию на основе проблемы
		a.AddRecommendation(fmt.Sprintf("Улучшите '%s': %s", audit.Title, audit.Description))
	}
}

// addCategoryRecommendation добавляет общую рекомендацию для категории на основе оценки
func (a *LighthouseAnalyzer) addCategoryRecommendation(category string, score float64) {
	if score >= 0.9 {
		return // Не добавляем рекомендации для хороших категорий
	}

	var recommendation string

	switch category {
	case "performance":
		recommendation = "Улучшите общую производительность страницы. Оптимизируйте загрузку ресурсов, уменьшите размер JavaScript и CSS, используйте кеширование."
	case "accessibility":
		recommendation = "Улучшите доступность страницы. Убедитесь, что контент доступен для всех пользователей, включая тех, кто использует вспомогательные технологии."
	case "best-practices":
		recommendation = "Следуйте современным веб-стандартам и лучшим практикам. Обновите устаревшие библиотеки, исправьте ошибки в консоли."
	case "seo":
		recommendation = "Оптимизируйте страницу для поисковых систем. Улучшите мета-описания, заголовки, и структуру контента."
	default:
		recommendation = fmt.Sprintf("Улучшите показатели категории '%s'.", category)
	}

	if score < 0.5 {
		recommendation = "Срочно " + recommendation
	}

	a.AddRecommendation(recommendation)
}

// GetPerformanceMetricsString возвращает форматированную строку с метриками производительности
func (a *LighthouseAnalyzer) GetPerformanceMetricsString() string {
	metrics, ok := a.GetMetrics()["performance_metrics"]
	if !ok {
		return "Метрики производительности недоступны"
	}

	metricsMap, ok := metrics.(lighthouse.MetricsResult)
	if !ok {
		return "Неверный формат метрик производительности"
	}

	result := "Метрики производительности:\n"

	// Core Web Vitals с выделением статуса
	lcpStatus := "✅"
	if metricsMap.LargestContentfulPaint > LCPGood {
		if metricsMap.LargestContentfulPaint > LCPNeedsImprovement {
			lcpStatus = "❌"
		} else {
			lcpStatus = "⚠️"
		}
	}

	clsStatus := "✅"
	if metricsMap.CumulativeLayoutShift > CLSGood {
		if metricsMap.CumulativeLayoutShift > CLSNeedsImprovement {
			clsStatus = "❌"
		} else {
			clsStatus = "⚠️"
		}
	}

	tbtStatus := "✅"
	if metricsMap.TotalBlockingTime > TBTGood {
		if metricsMap.TotalBlockingTime > TBTNeedsImprovement {
			tbtStatus = "❌"
		} else {
			tbtStatus = "⚠️"
		}
	}

	// Core Web Vitals
	result += fmt.Sprintf("- Largest Contentful Paint (LCP): %.1f ms %s\n", metricsMap.LargestContentfulPaint, lcpStatus)
	result += fmt.Sprintf("- Cumulative Layout Shift (CLS): %.3f %s\n", metricsMap.CumulativeLayoutShift, clsStatus)
	result += fmt.Sprintf("- Total Blocking Time (TBT): %.1f ms %s\n", metricsMap.TotalBlockingTime, tbtStatus)

	// Другие важные метрики
	result += fmt.Sprintf("- First Contentful Paint: %.1f ms\n", metricsMap.FirstContentfulPaint)
	result += fmt.Sprintf("- Time to Interactive: %.1f ms\n", metricsMap.TimeToInteractive)
	result += fmt.Sprintf("- Speed Index: %.1f ms\n", metricsMap.SpeedIndex)
	result += fmt.Sprintf("- First Meaningful Paint: %.1f ms\n", metricsMap.FirstMeaningfulPaint)

	// Дополнительная информация
	if a.GetMetrics()["dom_size"] != nil {
		result += fmt.Sprintf("- DOM Size: %d элементов\n", metricsMap.DOMSize)
	}

	if a.GetMetrics()["network_requests"] != nil {
		result += fmt.Sprintf("- Network Requests: %d\n", metricsMap.NetworkRequests)
	}

	if a.GetMetrics()["total_byte_weight"] != nil {
		result += fmt.Sprintf("- Total Page Size: %.1f КБ\n", float64(metricsMap.TotalByteWeight)/1024)
	}

	return result
}

// GetCoreWebVitalsStatus возвращает общий статус Core Web Vitals
func (a *LighthouseAnalyzer) GetCoreWebVitalsStatus() string {
	status, ok := a.GetMetrics()["core_web_vitals_status"]
	if !ok {
		return "unknown"
	}

	return status.(string)
}

// ForceRefresh принудительно обновляет анализ, игнорируя кеш
func (a *LighthouseAnalyzer) ForceRefresh(ctx context.Context, data *parser.WebsiteData) (map[string]interface{}, error) {
	// Временно отключаем кеширование
	oldCacheSetting := a.DisableCaching
	a.DisableCaching = true

	// Выполняем анализ
	result, err := a.Analyze(ctx, data, nil)

	// Восстанавливаем настройку кеширования
	a.DisableCaching = oldCacheSetting

	return result, err
}
