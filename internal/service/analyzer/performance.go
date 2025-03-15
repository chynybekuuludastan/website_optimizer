package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// PerformanceAnalyzer анализирует аспекты производительности веб-сайта
type PerformanceAnalyzer struct {
	*BaseAnalyzer
}

// NewPerformanceAnalyzer создает новый анализатор производительности
func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(PerformanceType),
	}
}

// Analyze выполняет анализ производительности на данных веб-сайта
func (a *PerformanceAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Проверяем, доступны ли результаты Lighthouse
	lighthouseUsed := false
	if lighthouseResults, ok := prevResults[LighthouseType]; ok {
		// Используем оценку производительности из Lighthouse
		if categoryScores, ok := lighthouseResults["category_scores"].(map[string]float64); ok {
			if perfScore, ok := categoryScores["performance"]; ok {
				a.SetMetric("lighthouse_performance_score", perfScore*100)
				lighthouseUsed = true
			}
		}

		// Используем метрики производительности из Lighthouse
		if metrics, ok := lighthouseResults["performance_metrics"].(lighthouse.MetricsResult); ok {
			a.SetMetric("lighthouse_metrics", metrics)

			// Добавляем результаты на основе метрик
			if metrics.FirstContentfulPaint > 0 {
				a.SetMetric("first_contentful_paint", metrics.FirstContentfulPaint)

				if metrics.FirstContentfulPaint > 2000 {
					a.AddIssue(map[string]interface{}{
						"type":        "slow_fcp",
						"severity":    "medium",
						"description": "Медленный First Contentful Paint",
						"value":       metrics.FirstContentfulPaint,
						"threshold":   2000,
					})
					a.AddRecommendation("Оптимизируйте First Contentful Paint для улучшения восприятия загрузки")
				}
			}

			if metrics.LargestContentfulPaint > 0 {
				a.SetMetric("largest_contentful_paint", metrics.LargestContentfulPaint)

				if metrics.LargestContentfulPaint > 2500 {
					a.AddIssue(map[string]interface{}{
						"type":        "slow_lcp",
						"severity":    "high",
						"description": "Медленный Largest Contentful Paint",
						"value":       metrics.LargestContentfulPaint,
						"threshold":   2500,
					})
					a.AddRecommendation("Оптимизируйте Largest Contentful Paint (LCP) для улучшения восприятия загрузки")
				}
			}

			// Дополнительные проверки TBT, CLS и других метрик...
		}

		// Используем аудиты Lighthouse, относящиеся к производительности
		if audits, ok := lighthouseResults["audits"].(map[string]lighthouse.Audit); ok {
			// Фильтруем только аудиты, относящиеся к производительности
			// и которые ещё не обработаны
			perfAudits := filterPerformanceAudits(audits)
			if len(perfAudits) > 0 {
				a.SetMetric("lighthouse_performance_audits", perfAudits)
			}
		}
	}

	// Базовые метрики, которые мы всё равно хотим оценить
	loadTimeSeconds := float64(data.LoadTime.Milliseconds()) / 1000.0
	totalPageSizeBytes := int64(len(data.HTML))
	numRequests := len(data.Images) + len(data.Scripts) + len(data.Styles) + 1 // +1 для основного HTML

	a.SetMetric("load_time_seconds", loadTimeSeconds)
	a.SetMetric("total_page_size_bytes", totalPageSizeBytes)
	a.SetMetric("num_requests", numRequests)

	// Проверки, которые не охвачены Lighthouse
	if !lighthouseUsed || totalPageSizeBytes > 2*1024*1024 {
		a.analyzePageSize(totalPageSizeBytes)
	}

	if !lighthouseUsed {
		// Проверка больших изображений
		a.analyzeLargeImages(data)

		// Проверка ресурсов, блокирующих рендеринг
		a.analyzeRenderBlockingResources(data)

		// Проверка неминифицированных CSS и JS
		a.analyzeMinification(data)

		// Проверка встроенных стилей
		a.analyzeInlineStyles(data)

		// Оценка времени загрузки
		a.analyzeLoadTime(loadTimeSeconds)

		// Оценка количества запросов
		a.analyzeRequestCount(numRequests)
	}

	// Расчет оценки производительности
	var score float64

	// Если есть оценка Lighthouse, учитываем ее с большим весом
	if lighthouseScore, ok := a.GetMetrics()["lighthouse_performance_score"].(float64); ok && lighthouseUsed {
		// Lighthouse дает результат с весом 80%, наш анализ - 20%
		score = lighthouseScore*0.8 + a.CalculateScore()*0.2
	} else {
		score = a.CalculateScore()

		// Корректировка на основе времени загрузки
		if loadTimeSeconds > 0 {
			if loadTimeSeconds < 1.0 {
				score += 10
			} else if loadTimeSeconds > 5.0 {
				score -= 20
			} else if loadTimeSeconds > 3.0 {
				score -= 10
			}
		}
	}

	// Нормализация оценки
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// filterPerformanceAudits фильтрует аудиты Lighthouse по категории производительности
func filterPerformanceAudits(audits map[string]lighthouse.Audit) map[string]lighthouse.Audit {
	perfAudits := make(map[string]lighthouse.Audit)

	// Список аудитов, относящихся к производительности
	perfAuditIDs := map[string]bool{
		"first-contentful-paint":    true,
		"largest-contentful-paint":  true,
		"speed-index":               true,
		"total-blocking-time":       true,
		"interactive":               true,
		"cumulative-layout-shift":   true,
		"server-response-time":      true,
		"render-blocking-resources": true,
		"unminified-css":            true,
		"unminified-javascript":     true,
		"unused-css-rules":          true,
		"unused-javascript":         true,
		"uses-optimized-images":     true,
		"uses-webp-images":          true,
		"uses-text-compression":     true,
		"uses-responsive-images":    true,
		"efficiently-encode-images": true,
		"dom-size":                  true,
	}

	for id, audit := range audits {
		if perfAuditIDs[id] {
			perfAudits[id] = audit
		}
	}

	return perfAudits
}

// analyzeLargeImages проверяет наличие больших изображений
func (a *PerformanceAnalyzer) analyzeLargeImages(data *parser.WebsiteData) {
	largeImages := []map[string]interface{}{}

	for _, img := range data.Images {
		if img.FileSize > 100000 { // порог в 100 КБ
			largeImages = append(largeImages, map[string]interface{}{
				"url":  img.URL,
				"size": img.FileSize,
			})
			a.AddIssue(map[string]interface{}{
				"type":        "large_image",
				"severity":    "medium",
				"description": "Изображение слишком большого размера",
				"url":         img.URL,
				"size":        img.FileSize,
			})
		}
	}

	a.SetMetric("large_images", largeImages)

	if len(largeImages) > 0 {
		a.AddRecommendation("Оптимизируйте большие изображения для улучшения времени загрузки страницы")
	}
}

// analyzeRenderBlockingResources проверяет ресурсы, блокирующие рендеринг
func (a *PerformanceAnalyzer) analyzeRenderBlockingResources(data *parser.WebsiteData) {
	renderBlockingAssets := []string{}

	for _, script := range data.Scripts {
		// Проверка, находится ли скрипт в head и не имеет async/defer
		if !strings.Contains(script.URL, "async") && !strings.Contains(script.URL, "defer") && !script.IsAsync && !script.IsDeferred {
			renderBlockingAssets = append(renderBlockingAssets, script.URL)
			a.AddIssue(map[string]interface{}{
				"type":        "render_blocking_script",
				"severity":    "medium",
				"description": "Скрипт может блокировать рендеринг",
				"url":         script.URL,
			})
		}
	}

	a.SetMetric("render_blocking_assets", renderBlockingAssets)

	if len(renderBlockingAssets) > 0 {
		a.AddRecommendation("Добавьте атрибуты async или defer к скриптам, блокирующим рендеринг")
	}
}

// analyzeMinification проверяет минификацию CSS и JS
func (a *PerformanceAnalyzer) analyzeMinification(data *parser.WebsiteData) {
	unminifiedCSS := []string{}
	unminifiedJS := []string{}

	// Проверка CSS на минификацию (на основе имени файла)
	for _, style := range data.Styles {
		if strings.Contains(style.URL, ".min.css") {
			continue // Уже минифицирован
		}
		unminifiedCSS = append(unminifiedCSS, style.URL)
		a.AddIssue(map[string]interface{}{
			"type":        "unminified_css",
			"severity":    "low",
			"description": "CSS-файл может быть не минифицирован",
			"url":         style.URL,
		})
	}

	// Проверка JS на минификацию (на основе имени файла)
	for _, script := range data.Scripts {
		if strings.Contains(script.URL, ".min.js") {
			continue // Уже минифицирован
		}
		unminifiedJS = append(unminifiedJS, script.URL)
		a.AddIssue(map[string]interface{}{
			"type":        "unminified_js",
			"severity":    "low",
			"description": "JavaScript-файл может быть не минифицирован",
			"url":         script.URL,
		})
	}

	a.SetMetric("unminified_css", unminifiedCSS)
	a.SetMetric("unminified_js", unminifiedJS)

	if len(unminifiedCSS) > 0 {
		a.AddRecommendation("Минифицируйте CSS-файлы для уменьшения размера")
	}

	if len(unminifiedJS) > 0 {
		a.AddRecommendation("Минифицируйте JavaScript-файлы для уменьшения размера")
	}
}

// analyzeInlineStyles проверяет наличие встроенных стилей
func (a *PerformanceAnalyzer) analyzeInlineStyles(data *parser.WebsiteData) {
	inlineStyleRegex := regexp.MustCompile(`<style\b[^>]*>(.*?)</style>`)
	if inlineStyleRegex.MatchString(data.HTML) {
		a.AddIssue(map[string]interface{}{
			"type":        "inline_css",
			"severity":    "low",
			"description": "Страница содержит встроенные CSS, которые могут блокировать рендеринг",
		})
		a.AddRecommendation("Переместите встроенные CSS во внешние таблицы стилей")
	}
}

// analyzeLoadTime оценивает время загрузки
func (a *PerformanceAnalyzer) analyzeLoadTime(loadTime float64) {
	if loadTime > 3.0 {
		a.AddIssue(map[string]interface{}{
			"type":        "slow_load_time",
			"severity":    "high",
			"description": "Время загрузки страницы слишком долгое",
			"load_time":   loadTime,
			"threshold":   3.0,
		})
		a.AddRecommendation("Улучшите время загрузки страницы для улучшения пользовательского опыта")
	}
}

// analyzePageSize оценивает размер страницы
func (a *PerformanceAnalyzer) analyzePageSize(pageSize int64) {
	if pageSize > 2*1024*1024 { // порог в 2 МБ
		a.AddIssue(map[string]interface{}{
			"type":        "large_page_size",
			"severity":    "medium",
			"description": "Общий размер страницы слишком большой",
			"size":        pageSize,
			"threshold":   2 * 1024 * 1024,
		})
		a.AddRecommendation("Уменьшите общий размер страницы для улучшения времени загрузки")
	}
}

// analyzeRequestCount оценивает количество HTTP-запросов
func (a *PerformanceAnalyzer) analyzeRequestCount(count int) {
	if count > 50 {
		a.AddIssue(map[string]interface{}{
			"type":        "too_many_requests",
			"severity":    "medium",
			"description": "Страница делает слишком много HTTP-запросов",
			"count":       count,
			"threshold":   50,
		})
		a.AddRecommendation("Уменьшите количество HTTP-запросов путем объединения ресурсов")
	}
}

// LighthouseIntegration представляет интеграцию с Lighthouse API
type LighthouseIntegration struct {
	LighthouseURL string
	APIKey        string
	Timeout       time.Duration
}

// LighthouseRequest представляет запрос к Lighthouse API
type LighthouseRequest struct {
	URL     string `json:"url"`
	Options struct {
		FormFactor string   `json:"formFactor"`
		Categories []string `json:"categories"`
	} `json:"options"`
}

// LighthouseResponse представляет ответ от Lighthouse API
type LighthouseResponse struct {
	Status     string `json:"status"`
	ID         string `json:"id,omitempty"`
	Error      string `json:"error,omitempty"`
	Categories struct {
		Performance struct {
			Score  float64 `json:"score"`
			Audits map[string]struct {
				Title        string  `json:"title"`
				Description  string  `json:"description"`
				Score        float64 `json:"score"`
				NumericValue float64 `json:"numericValue,omitempty"`
			} `json:"audits"`
		} `json:"performance"`
	} `json:"categories,omitempty"`
}

// NewLighthouseIntegration создает новую интеграцию с Lighthouse
func NewLighthouseIntegration(lighthouseURL, apiKey string) *LighthouseIntegration {
	return &LighthouseIntegration{
		LighthouseURL: lighthouseURL,
		APIKey:        apiKey,
		Timeout:       60 * time.Second,
	}
}

// RunAudit запускает аудит Lighthouse и возвращает результаты
func (l *LighthouseIntegration) RunAudit(url string, categories []string) (*LighthouseResponse, error) {
	req := LighthouseRequest{
		URL: url,
	}
	req.Options.FormFactor = "desktop"
	req.Options.Categories = categories

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", l.LighthouseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if l.APIKey != "" {
		httpReq.Header.Set("X-API-Key", l.APIKey)
	}

	client := &http.Client{
		Timeout: l.Timeout,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lighthouse API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var lightResp LighthouseResponse
	err = json.Unmarshal(body, &lightResp)
	if err != nil {
		return nil, err
	}

	return &lightResp, nil
}

// AnalyzeWithLighthouse выполняет анализ с помощью Lighthouse
func (a *PerformanceAnalyzer) AnalyzeWithLighthouse(url string, lighthouseIntegration *LighthouseIntegration) (map[string]interface{}, error) {
	categories := []string{"performance"}

	lighthouseResults, err := lighthouseIntegration.RunAudit(url, categories)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запуске аудита Lighthouse: %v", err)
	}

	if lighthouseResults.Status != "success" {
		return nil, fmt.Errorf("аудит Lighthouse завершился неудачно: %s", lighthouseResults.Error)
	}

	// Добавляем результаты Lighthouse в метрики
	a.SetMetric("lighthouse_score", lighthouseResults.Categories.Performance.Score*100)

	audits := make(map[string]interface{})
	for name, audit := range lighthouseResults.Categories.Performance.Audits {
		audits[name] = map[string]interface{}{
			"title":         audit.Title,
			"description":   audit.Description,
			"score":         audit.Score,
			"numeric_value": audit.NumericValue,
		}

		// Добавляем проблемы на основе аудитов Lighthouse
		if audit.Score < 0.5 && audit.Score > 0 {
			a.AddIssue(map[string]interface{}{
				"type":        "lighthouse_" + name,
				"severity":    "medium",
				"description": audit.Title,
				"details":     audit.Description,
				"score":       audit.Score,
			})

			// Добавляем рекомендацию
			a.AddRecommendation(fmt.Sprintf("Улучшите '%s': %s", audit.Title, audit.Description))
		}
	}

	a.SetMetric("lighthouse_audits", audits)

	return a.GetMetrics(), nil
}
