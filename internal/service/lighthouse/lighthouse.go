package lighthouse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Category представляет категорию аудита Lighthouse
type Category string

const (
	CategoryPerformance   Category = "performance"
	CategoryAccessibility Category = "accessibility"
	CategoryBestPractices Category = "best-practices"
	CategorySEO           Category = "seo"
	CategoryPWA           Category = "pwa"
)

// FormFactor представляет формфактор устройства для аудита
type FormFactor string

const (
	FormFactorMobile  FormFactor = "mobile"
	FormFactorDesktop FormFactor = "desktop"
)

// AuditOptions представляет опции для аудита Lighthouse
type AuditOptions struct {
	Categories []Category
	FormFactor FormFactor
	Locale     string
	Strategy   string
	URLShardId string
}

// MetricsResult представляет конкретные метрики производительности
type MetricsResult struct {
	FirstContentfulPaint   float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint float64 `json:"largestContentfulPaint"`
	FirstMeaningfulPaint   float64 `json:"firstMeaningfulPaint"`
	SpeedIndex             float64 `json:"speedIndex"`
	TotalBlockingTime      float64 `json:"totalBlockingTime"`
	MaxPotentialFID        float64 `json:"maxPotentialFID"`
	CumulativeLayoutShift  float64 `json:"cumulativeLayoutShift"`
	TimeToInteractive      float64 `json:"interactive"`
	FirstCPUIdle           float64 `json:"firstCPUIdle"`
}

// Client представляет клиент для Lighthouse API
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// AnalysisResult представляет структурированные результаты анализа
type AnalysisResult struct {
	Scores            map[string]float64
	Metrics           MetricsResult
	Audits            map[string]Audit
	Issues            []map[string]interface{}
	Recommendations   []string
	LighthouseVersion string
	FetchTime         string
	TotalAnalysisTime float64
}

// PageSpeedResponse представляет ответ от Google PageSpeed API
type PageSpeedResponse struct {
	CaptchaResult     string `json:"captchaResult"`
	Kind              string `json:"kind"`
	ID                string `json:"id"`
	LoadingExperience struct {
		ID               string `json:"id"`
		MetricsEqualHost bool   `json:"metricsEqualHost"`
		Overall          struct {
			Category string `json:"category"`
		} `json:"overall"`
	} `json:"loadingExperience"`
	OriginLoadingExperience struct {
		ID               string `json:"id"`
		MetricsEqualHost bool   `json:"metricsEqualHost"`
		Overall          struct {
			Category string `json:"category"`
		} `json:"overall"`
	} `json:"originLoadingExperience"`
	LighthouseResult struct {
		RequestedURL      string `json:"requestedUrl"`
		FinalURL          string `json:"finalUrl"`
		LighthouseVersion string `json:"lighthouseVersion"`
		UserAgent         string `json:"userAgent"`
		FetchTime         string `json:"fetchTime"`
		Environment       struct {
			NetworkUserAgent string                 `json:"networkUserAgent"`
			HostUserAgent    string                 `json:"hostUserAgent"`
			BenchmarkIndex   float64                `json:"benchmarkIndex"`
			Credits          map[string]interface{} `json:"credits"`
		} `json:"environment"`
		RunWarnings    []interface{}             `json:"runWarnings"`
		ConfigSettings map[string]interface{}    `json:"configSettings"`
		Categories     map[string]CategoryResult `json:"categories"`
		CategoryGroups map[string]interface{}    `json:"categoryGroups"`
		Audits         map[string]Audit          `json:"audits"`
		Timing         struct {
			Total float64 `json:"total"`
		} `json:"timing"`
		I18n map[string]interface{} `json:"i18n"`
	} `json:"lighthouseResult"`
	AnalysisUTCTimestamp string `json:"analysisUTCTimestamp"`
}

// CategoryResult представляет категорию в результатах Lighthouse
type CategoryResult struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Score             float64    `json:"score"`
	ManualDescription string     `json:"manualDescription"`
	AuditRefs         []AuditRef `json:"auditRefs"`
}

// AuditRef содержит ссылку на аудит
type AuditRef struct {
	ID     string  `json:"id"`
	Weight float64 `json:"weight"`
	Group  string  `json:"group,omitempty"`
}

// Audit представляет результат отдельного аудита Lighthouse
type Audit struct {
	ID               string      `json:"id"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	Score            float64     `json:"score"`
	ScoreDisplayMode string      `json:"scoreDisplayMode"`
	NumericValue     float64     `json:"numericValue,omitempty"`
	NumericUnit      string      `json:"numericUnit,omitempty"`
	DisplayValue     string      `json:"displayValue,omitempty"`
	Warnings         []string    `json:"warnings,omitempty"`
	Details          interface{} `json:"details,omitempty"`
}

// DefaultAuditOptions возвращает опции по умолчанию
func DefaultAuditOptions() AuditOptions {
	return AuditOptions{
		Categories: []Category{CategoryPerformance},
		FormFactor: FormFactorDesktop,
		Locale:     "ru",
		Strategy:   "",
		URLShardId: "",
	}
}

// NewClient создает новый клиент для Lighthouse API
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"
	}

	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// RunAudit запускает аудит Lighthouse для указанного URL
func (c *Client) RunAudit(ctx context.Context, targetURL string, options AuditOptions) (*PageSpeedResponse, error) {
	// Создаем базовый URL
	apiURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("неверный базовый URL: %w", err)
	}

	// Добавляем параметры запроса
	query := apiURL.Query()
	query.Add("url", targetURL)

	// Добавляем API ключ, если он указан
	if c.APIKey != "" {
		query.Add("key", c.APIKey)
	}

	// Добавляем категории - ВАЖНО: каждая категория добавляется как отдельный параметр
	if len(options.Categories) > 0 {
		// Важно! Google PageSpeed API требует отдельные параметры для каждой категории
		for _, cat := range options.Categories {
			query.Add("category", string(cat))
		}
	}

	// Добавляем формфактор (стратегию)
	if options.FormFactor != "" {
		query.Add("strategy", strings.ToLower(string(options.FormFactor)))
	}

	// Добавляем локаль
	if options.Locale != "" {
		query.Add("locale", options.Locale)
	}

	// Добавляем дополнительные параметры, если они указаны
	if options.Strategy != "" {
		query.Add("utm_source", options.Strategy)
	}

	if options.URLShardId != "" {
		query.Add("urlShardId", options.URLShardId)
	}

	// Устанавливаем параметры запроса
	apiURL.RawQuery = query.Encode()

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	// Выполняем запрос
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP-запроса: %w", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус код
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API вернул код состояния %d: %s", resp.StatusCode, string(body))
	}

	// Разбираем ответ
	var response PageSpeedResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON ответа: %w", err)
	}

	return &response, nil
}

// AnalyzeURL проводит полный анализ URL с преобразованием результатов
func (c *Client) AnalyzeURL(ctx context.Context, url string, options AuditOptions) (*AnalysisResult, error) {
	// Запускаем аудит
	response, err := c.RunAudit(ctx, url, options)
	if err != nil {
		return nil, err
	}

	// Преобразуем результаты
	result := &AnalysisResult{
		Scores:            make(map[string]float64),
		Audits:            response.LighthouseResult.Audits,
		LighthouseVersion: response.LighthouseResult.LighthouseVersion,
		FetchTime:         response.LighthouseResult.FetchTime,
		TotalAnalysisTime: response.LighthouseResult.Timing.Total,
	}

	// Заполняем оценки категорий
	for id, category := range response.LighthouseResult.Categories {
		result.Scores[id] = category.Score * 100
	}

	// Извлекаем метрики производительности
	if metricsAudit, ok := response.LighthouseResult.Audits["metrics"]; ok {
		if details, ok := metricsAudit.Details.(map[string]interface{}); ok {
			if items, ok := details["items"].([]interface{}); ok && len(items) > 0 {
				if firstItem, ok := items[0].(map[string]interface{}); ok {
					// Извлекаем отдельные метрики
					if fcp, ok := firstItem["firstContentfulPaint"].(float64); ok {
						result.Metrics.FirstContentfulPaint = fcp
					}
					if lcp, ok := firstItem["largestContentfulPaint"].(float64); ok {
						result.Metrics.LargestContentfulPaint = lcp
					}
					if fmp, ok := firstItem["firstMeaningfulPaint"].(float64); ok {
						result.Metrics.FirstMeaningfulPaint = fmp
					}
					if si, ok := firstItem["speedIndex"].(float64); ok {
						result.Metrics.SpeedIndex = si
					}
					if tbt, ok := firstItem["totalBlockingTime"].(float64); ok {
						result.Metrics.TotalBlockingTime = tbt
					}
					if mpfid, ok := firstItem["maxPotentialFID"].(float64); ok {
						result.Metrics.MaxPotentialFID = mpfid
					}
					if cls, ok := firstItem["cumulativeLayoutShift"].(float64); ok {
						result.Metrics.CumulativeLayoutShift = cls
					}
					if tti, ok := firstItem["interactive"].(float64); ok {
						result.Metrics.TimeToInteractive = tti
					}
					if idle, ok := firstItem["firstCPUIdle"].(float64); ok {
						result.Metrics.FirstCPUIdle = idle
					}
				}
			}
		}
	}

	// Извлекаем проблемы и рекомендации
	issues, recommendations := extractIssuesAndRecommendations(response)
	result.Issues = issues
	result.Recommendations = recommendations

	return result, nil
}

// extractIssuesAndRecommendations извлекает проблемы и рекомендации из результатов аудита
func extractIssuesAndRecommendations(response *PageSpeedResponse) ([]map[string]interface{}, []string) {
	issues := []map[string]interface{}{}
	recommendations := []string{}

	// Обрабатываем все аудиты
	for id, audit := range response.LighthouseResult.Audits {
		// Пропускаем информационные аудиты без оценок
		if audit.ScoreDisplayMode == "informative" || audit.ScoreDisplayMode == "notApplicable" {
			continue
		}

		// Если оценка ниже 0.5, считаем это проблемой
		if audit.Score < 0.5 && audit.Score >= 0 {
			severity := "medium"
			if audit.Score < 0.3 {
				severity = "high"
			} else if audit.Score >= 0.4 {
				severity = "low"
			}

			issue := map[string]interface{}{
				"type":        "lighthouse_" + id,
				"severity":    severity,
				"description": audit.Title,
				"details":     audit.Description,
				"score":       audit.Score,
			}

			if audit.DisplayValue != "" {
				issue["display_value"] = audit.DisplayValue
			}

			issues = append(issues, issue)
			recommendations = append(recommendations,
				fmt.Sprintf("Улучшите '%s': %s", audit.Title, audit.Description))
		}
	}

	return issues, recommendations
}

// ToMap преобразует результаты анализа в карту для простой сериализации
func (r *AnalysisResult) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"scores":             r.Scores,
		"metrics":            r.Metrics,
		"lighthouse_version": r.LighthouseVersion,
		"fetch_time":         r.FetchTime,
		"total_time":         r.TotalAnalysisTime,
		"issues":             r.Issues,
		"recommendations":    r.Recommendations,
	}

	// Добавляем только важные аудиты для экономии места
	importantAudits := map[string]interface{}{}
	for id, audit := range r.Audits {
		if audit.Score < 1 && audit.ScoreDisplayMode != "informative" && audit.ScoreDisplayMode != "notApplicable" {
			importantAudits[id] = map[string]interface{}{
				"title":         audit.Title,
				"description":   audit.Description,
				"score":         audit.Score,
				"display_value": audit.DisplayValue,
			}
		}
	}
	result["important_audits"] = importantAudits

	return result
}
