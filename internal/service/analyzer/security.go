package analyzer

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// SecurityAnalyzer анализирует аспекты безопасности веб-сайта
type SecurityAnalyzer struct {
	*BaseAnalyzer
}

// NewSecurityAnalyzer создает новый анализатор безопасности
func NewSecurityAnalyzer() *SecurityAnalyzer {
	return &SecurityAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
	}
}

// Analyze выполняет анализ безопасности веб-сайта
func (a *SecurityAnalyzer) Analyze(data *parser.WebsiteData) (map[string]interface{}, error) {
	// Анализ с помощью goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
	if err != nil {
		return a.GetMetrics(), err
	}

	// Проверка HTTPS
	a.analyzeHTTPS(data)

	// Проверка заголовков безопасности
	a.analyzeSecurityHeaders(data)

	// Проверка смешанного контента
	a.analyzeMixedContent(data)

	// Проверка защиты от CSRF
	a.analyzeCSRF(doc)

	// Проверка наличия встроенного JavaScript
	a.analyzeInlineJS(data)

	// Проверка наличия устаревших, небезопасных API
	a.analyzeDeprecatedAPIs(data)

	// Проверка наличия метаданных, указывающих на X-Frame-Options
	a.analyzeXFrameOptions(data)

	// Расчет общей оценки
	score := a.CalculateScore()

	// HTTPS имеет большое влияние на оценку
	if !a.GetMetrics()["has_https"].(bool) {
		score -= 30
	}

	// Убедимся, что оценка между 0 и 100
	if score < 0 {
		score = 0
	}
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeHTTPS проверяет использование HTTPS
func (a *SecurityAnalyzer) analyzeHTTPS(data *parser.WebsiteData) {
	parsedURL, err := url.Parse(data.URL)
	hasHTTPS := false

	if err == nil {
		hasHTTPS = parsedURL.Scheme == "https"
	}

	a.SetMetric("has_https", hasHTTPS)

	if !hasHTTPS {
		a.AddIssue(map[string]interface{}{
			"type":        "no_https",
			"severity":    "high",
			"description": "Сайт не использует HTTPS",
		})
		a.AddRecommendation("Включите HTTPS для защиты передачи данных")
	}
}

// analyzeSecurityHeaders проверяет заголовки безопасности
func (a *SecurityAnalyzer) analyzeSecurityHeaders(data *parser.WebsiteData) {
	// В реальной реализации эти данные будут из HTTP-заголовков ответа
	// Для этого примера мы определим их из HTML-мета-тегов или предположим, что они отсутствуют

	missingSecHeaders := []string{}

	// Проверка Content-Security-Policy
	hasCSP := false
	for key, value := range data.MetaTags {
		if key == "content-security-policy" && value != "" {
			hasCSP = true
			break
		}
	}
	a.SetMetric("has_content_security_policy", hasCSP)

	if !hasCSP {
		missingSecHeaders = append(missingSecHeaders, "Content-Security-Policy")
		a.AddIssue(map[string]interface{}{
			"type":        "missing_csp",
			"severity":    "medium",
			"description": "Content Security Policy не реализована",
		})
		a.AddRecommendation("Реализуйте Content Security Policy для предотвращения XSS-атак")
	}

	// Проверка X-XSS-Protection
	hasXSSProtection := false
	for key, value := range data.MetaTags {
		if key == "x-xss-protection" && value != "" {
			hasXSSProtection = true
			break
		}
	}
	a.SetMetric("has_xss_protection", hasXSSProtection)

	if !hasXSSProtection {
		missingSecHeaders = append(missingSecHeaders, "X-XSS-Protection")
		a.AddIssue(map[string]interface{}{
			"type":        "missing_xss_protection",
			"severity":    "medium",
			"description": "Заголовок X-XSS-Protection не реализован",
		})
		a.AddRecommendation("Добавьте заголовок X-XSS-Protection для защиты от XSS-атак")
	}

	// Проверка HSTS
	hasHSTS := false
	for key, value := range data.MetaTags {
		if key == "strict-transport-security" && value != "" {
			hasHSTS = true
			break
		}
	}
	a.SetMetric("has_hsts", hasHSTS)

	if !hasHSTS && a.GetMetrics()["has_https"].(bool) {
		missingSecHeaders = append(missingSecHeaders, "Strict-Transport-Security")
		a.AddIssue(map[string]interface{}{
			"type":        "missing_hsts",
			"severity":    "medium",
			"description": "HTTP Strict Transport Security не реализован",
		})
		a.AddRecommendation("Реализуйте HSTS для принудительного использования безопасных соединений")
	}

	a.SetMetric("missing_security_headers", missingSecHeaders)
}

// analyzeMixedContent проверяет наличие смешанного контента
func (a *SecurityAnalyzer) analyzeMixedContent(data *parser.WebsiteData) {
	mixedContent := []string{}

	if a.GetMetrics()["has_https"].(bool) {
		// Проверка HTTP-ресурсов на HTTPS-странице
		for _, img := range data.Images {
			if strings.HasPrefix(img.URL, "http:") {
				mixedContent = append(mixedContent, img.URL)
				a.AddIssue(map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Смешанный контент: HTTP-ресурс на HTTPS-странице",
					"url":         img.URL,
				})
			}
		}

		for _, script := range data.Scripts {
			if strings.HasPrefix(script.URL, "http:") {
				mixedContent = append(mixedContent, script.URL)
				a.AddIssue(map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Смешанный контент: HTTP-скрипт на HTTPS-странице",
					"url":         script.URL,
				})
			}
		}

		for _, style := range data.Styles {
			if strings.HasPrefix(style.URL, "http:") {
				mixedContent = append(mixedContent, style.URL)
				a.AddIssue(map[string]interface{}{
					"type":        "mixed_content",
					"severity":    "high",
					"description": "Смешанный контент: HTTP-стиль на HTTPS-странице",
					"url":         style.URL,
				})
			}
		}
	}

	a.SetMetric("mixed_content", mixedContent)

	if len(mixedContent) > 0 {
		a.AddRecommendation("Исправьте смешанный контент, обновив все ресурсы для использования HTTPS")
	}
}

// analyzeCSRF проверяет защиту от CSRF
func (a *SecurityAnalyzer) analyzeCSRF(doc *goquery.Document) {
	forms := doc.Find("form")
	formsWithoutCSRF := 0

	forms.Each(func(i int, s *goquery.Selection) {
		// Проверяем наличие CSRF-токена
		csrfFields := s.Find("input[name*='csrf'], input[name*='token'], input[name*='_token']")
		if csrfFields.Length() == 0 {
			formsWithoutCSRF++
		}
	})

	a.SetMetric("forms_without_csrf", formsWithoutCSRF)

	if forms.Length() > 0 && formsWithoutCSRF > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "possible_csrf_vulnerability",
			"severity":    "high",
			"description": "Формы найдены без очевидной CSRF-защиты",
			"count":       formsWithoutCSRF,
		})
		a.AddRecommendation("Реализуйте CSRF-токены для всех форм")
	}
}

// analyzeInlineJS проверяет наличие встроенного JavaScript
func (a *SecurityAnalyzer) analyzeInlineJS(data *parser.WebsiteData) {
	hasInlineJS := strings.Contains(data.HTML, "<script>") || strings.Contains(data.HTML, "javascript:")
	a.SetMetric("has_inline_js", hasInlineJS)

	if hasInlineJS {
		a.AddIssue(map[string]interface{}{
			"type":        "inline_js",
			"severity":    "medium",
			"description": "Найден встроенный JavaScript, который может быть угрозой безопасности",
		})
		a.AddRecommendation("Избегайте встроенного JavaScript и используйте внешние скрипты с CSP")
	}
}

// analyzeDeprecatedAPIs проверяет наличие устаревших, небезопасных API
func (a *SecurityAnalyzer) analyzeDeprecatedAPIs(data *parser.WebsiteData) {
	deprecatedAPIs := []string{}

	// Проверяем использование устаревших API
	if strings.Contains(data.HTML, "document.write") {
		deprecatedAPIs = append(deprecatedAPIs, "document.write")
	}

	if strings.Contains(data.HTML, "eval(") {
		deprecatedAPIs = append(deprecatedAPIs, "eval")
	}

	if strings.Contains(data.HTML, "localStorage") {
		// localStorage сам по себе не устаревший, но рискованный для хранения чувствительных данных
		deprecatedAPIs = append(deprecatedAPIs, "localStorage для чувствительных данных")
	}

	a.SetMetric("deprecated_apis", deprecatedAPIs)

	if len(deprecatedAPIs) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "deprecated_apis",
			"severity":    "medium",
			"description": "Использование устаревших или небезопасных API",
			"apis":        deprecatedAPIs,
		})
		a.AddRecommendation("Замените устаревшие и небезопасные API современными и безопасными альтернативами")
	}
}

// analyzeXFrameOptions проверяет настройку X-Frame-Options
func (a *SecurityAnalyzer) analyzeXFrameOptions(data *parser.WebsiteData) {
	hasXFrameOptions := false

	// Проверка мета-тегов на X-Frame-Options
	for key, value := range data.MetaTags {
		if key == "x-frame-options" && (value == "DENY" || value == "SAMEORIGIN") {
			hasXFrameOptions = true
			break
		}
	}

	a.SetMetric("has_x_frame_options", hasXFrameOptions)

	if !hasXFrameOptions {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_x_frame_options",
			"severity":    "medium",
			"description": "Отсутствует защита от кликджекинга (X-Frame-Options)",
		})
		a.AddRecommendation("Добавьте заголовок X-Frame-Options для предотвращения кликджекинга")
	}
}
