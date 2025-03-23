package analyzer

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// SEOAnalyzer анализирует SEO-аспекты веб-сайта
type SEOAnalyzer struct {
	*BaseAnalyzer
}

// NewSEOAnalyzer создает новый SEO-анализатор
func NewSEOAnalyzer() *SEOAnalyzer {
	return &SEOAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(SEOType),
	}
}

// Analyze выполняет SEO-анализ данных веб-сайта
func (a *SEOAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Проверка контекста
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Проверяем, доступны ли результаты Lighthouse
	lighthouseUsed := false
	titleChecked := false
	metaDescChecked := false

	if lighthouseResults, ok := prevResults[LighthouseType]; ok {
		// Используем SEO оценку из Lighthouse если она доступна
		if categoryScores, ok := lighthouseResults["category_scores"].(map[string]float64); ok {
			if seoScore, ok := categoryScores["seo"]; ok {
				a.SetMetric("lighthouse_seo_score", seoScore*100)
				lighthouseUsed = true
			}
		}

		// Определяем SEO-релевантные аудиты
		seoAuditTypes := map[string]bool{
			"meta-description": true,
			"document-title":   true,
			"hreflang":         true,
			"canonical":        true,
			"robots-txt":       true,
			"link-text":        true,
			"is-crawlable":     true,
			"image-alt":        true,
			"structured-data":  true,
		}

		// Извлекаем данные аудитов
		seoAudits := extractAuditsData(lighthouseResults, seoAuditTypes)

		if len(seoAudits) > 0 {
			a.SetMetric("lighthouse_seo_audits", seoAudits)

			// Проверяем наличие конкретных аудитов
			_, titleChecked = seoAudits["document-title"]
			_, metaDescChecked = seoAudits["meta-description"]

			// Получаем проблемы из аудитов
			seoIssues := getAuditIssues(seoAudits, 0.9)

			// Добавляем проблемы и рекомендации
			for _, issue := range seoIssues {
				a.AddIssue(issue)
				if description, ok := issue["details"].(string); ok {
					a.AddRecommendation(description)
				}
			}
		}
	}

	// Проверка мета-тегов title и description только если не проверено Lighthouse
	if !titleChecked || !metaDescChecked {
		a.analyzeTitleAndDescription(data)
	}

	a.analyzeHeadings(data)
	a.analyzeImages(data)
	a.analyzeLinks(data)
	a.analyzeCanonical(data)
	a.analyzeKeywords(data)

	// Расчет общей оценки
	var score float64

	if lighthouseScore, ok := a.GetMetrics()["lighthouse_seo_score"].(float64); ok && lighthouseUsed {
		score = lighthouseScore*0.7 + a.CalculateScore()*0.3
	} else {
		score = a.CalculateScore()
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

// getSeverityFromScore определяет уровень серьезности на основе оценки Lighthouse
func getSeverityFromScore(score float64) string {
	if score < 0.3 {
		return "high"
	} else if score < 0.7 {
		return "medium"
	}
	return "low"
}

// analyzeHeadings анализирует структуру заголовков
func (a *SEOAnalyzer) analyzeHeadings(data *parser.WebsiteData) {
	headingStructure := map[string]int{
		"h1": len(data.H1),
		"h2": len(data.H2),
		"h3": len(data.H3),
	}
	a.SetMetric("heading_structure", headingStructure)

	if len(data.H1) == 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_h1",
			"severity":    "high",
			"description": "На странице отсутствует заголовок H1",
		})
		a.AddRecommendation("Добавьте заголовок H1, который четко описывает содержание страницы")
	} else if len(data.H1) > 1 {
		a.AddIssue(map[string]interface{}{
			"type":        "multiple_h1",
			"severity":    "medium",
			"description": "На странице несколько заголовков H1",
			"count":       len(data.H1),
		})
		a.AddRecommendation("Используйте только один заголовок H1 на странице")
	}

	if len(data.H2) == 0 && len(data.TextContent) > 300 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_h2",
			"severity":    "medium",
			"description": "На странице отсутствуют заголовки H2",
		})
		a.AddRecommendation("Используйте заголовки H2 для структурирования контента")
	}
}

// analyzeLinks анализирует внутренние и внешние ссылки
func (a *SEOAnalyzer) analyzeLinks(data *parser.WebsiteData) {
	internalLinks := 0
	externalLinks := 0
	brokenLinks := []string{}

	for _, link := range data.Links {
		if link.IsInternal {
			internalLinks++
		} else {
			externalLinks++
		}

		if link.StatusCode >= 400 {
			brokenLinks = append(brokenLinks, link.URL)
		}
	}

	a.SetMetric("internal_links", internalLinks)
	a.SetMetric("external_links", externalLinks)
	a.SetMetric("broken_links", brokenLinks)

	if len(brokenLinks) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "broken_links",
			"severity":    "high",
			"description": "На странице есть неработающие ссылки",
			"count":       len(brokenLinks),
		})
		a.AddRecommendation("Исправьте или удалите неработающие ссылки на странице")
	}

	if internalLinks == 0 && len(data.Links) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "no_internal_links",
			"severity":    "medium",
			"description": "На странице нет внутренних ссылок",
		})
		a.AddRecommendation("Добавьте внутренние ссылки для улучшения навигации и индексации")
	}
}

// analyzeKeywords анализирует плотность ключевых слов
func (a *SEOAnalyzer) analyzeKeywords(data *parser.WebsiteData) {
	words := strings.Fields(strings.ToLower(data.TextContent))
	wordCount := make(map[string]int)
	totalWords := len(words)

	if totalWords == 0 {
		return
	}

	for _, word := range words {
		// Пропускаем короткие слова и стоп-слова
		if len(word) <= 2 || isStopWord(word) {
			continue
		}
		// Удаляем пунктуацию
		word = strings.Trim(word, ".,?!:;()")
		if word != "" {
			wordCount[word]++
		}
	}

	keywordDensity := make(map[string]float64)
	for word, count := range wordCount {
		if count >= 3 { // Учитываем только слова, которые появляются не менее 3 раз
			keywordDensity[word] = float64(count) / float64(totalWords) * 100
		}
	}
	a.SetMetric("keyword_density", keywordDensity)

	// Проверка на переоптимизацию
	highDensityKeywords := []string{}
	for word, density := range keywordDensity {
		if density > 5.0 {
			highDensityKeywords = append(highDensityKeywords, word)
		}
	}

	if len(highDensityKeywords) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "keyword_stuffing",
			"severity":    "medium",
			"description": "Возможная переоптимизация ключевых слов",
			"keywords":    highDensityKeywords,
		})
		a.AddRecommendation("Избегайте слишком частого использования ключевых слов")
	}
}

// isStopWord проверяет, является ли слово стоп-словом
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"а": true, "без": true, "более": true, "бы": true, "был": true, "была": true, "были": true, "было": true,
		"быть": true, "в": true, "вам": true, "вас": true, "весь": true, "во": true, "вот": true, "все": true,
		"всего": true, "всех": true, "вы": true, "где": true, "да": true, "даже": true, "для": true, "до": true,
		"его": true, "ее": true, "ей": true, "ему": true, "если": true, "есть": true, "еще": true, "же": true,
		"за": true, "здесь": true, "и": true, "из": true, "или": true, "им": true, "их": true, "к": true,
		"как": true, "ко": true, "когда": true, "кто": true, "ли": true, "либо": true, "мне": true, "может": true,
		"мы": true, "на": true, "надо": true, "наш": true, "не": true, "него": true, "нее": true, "нет": true,
		"ни": true, "них": true, "но": true, "ну": true, "о": true, "об": true, "однако": true, "он": true,
		"она": true, "они": true, "оно": true, "от": true, "очень": true, "по": true, "под": true, "при": true,
		"с": true, "со": true, "так": true, "также": true, "такой": true, "там": true, "те": true, "тем": true,
		"то": true, "того": true, "тоже": true, "той": true, "только": true, "том": true, "ты": true, "у": true,
		"уже": true, "хотя": true, "чего": true, "чей": true, "чем": true, "что": true, "чтобы": true, "эта": true,
		"эти": true, "это": true, "я": true,
		"the": true, "of": true, "and": true, "a": true, "to": true, "in": true, "is": true, "you": true,
		"that": true, "it": true, "he": true, "was": true, "for": true, "on": true, "are": true, "as": true,
		"with": true, "his": true, "they": true, "at": true, "be": true, "this": true, "have": true,
		"from": true, "or": true, "one": true, "had": true, "by": true, "but": true, "not": true, "what": true,
		"all": true, "were": true, "we": true, "when": true, "your": true, "can": true, "said": true, "there": true,
		"use": true, "an": true, "each": true, "which": true, "she": true, "do": true, "how": true, "their": true,
		"if": true, "will": true, "up": true, "other": true, "about": true, "out": true, "many": true, "then": true,
		"them": true, "these": true, "so": true, "some": true, "her": true, "would": true, "make": true, "like": true,
		"him": true, "into": true, "time": true, "has": true, "look": true, "two": true, "more": true, "go": true,
	}
	return stopWords[word]
}

// analyzeTitleAndDescription проверяет мета-теги title и description
func (a *SEOAnalyzer) analyzeTitleAndDescription(data *parser.WebsiteData) {
	// Проверка мета-тега title
	missingMetaTitle := data.Title == ""
	metaTitleLength := len(data.Title)

	a.SetMetric("missing_meta_title", missingMetaTitle)
	a.SetMetric("meta_title_length", metaTitleLength)

	// Анализ и рекомендации для title
	if missingMetaTitle {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_title",
			"severity":    "high",
			"description": "На странице отсутствует тег title",
		})
		a.AddRecommendation("Добавьте информативный title-тег на страницу")
	} else {
		// Проверка длины title
		if metaTitleLength < 30 {
			a.AddIssue(map[string]interface{}{
				"type":        "title_too_short",
				"severity":    "medium",
				"description": "Тег title слишком короткий",
				"current":     metaTitleLength,
				"recommended": "30-60 символов",
			})
			a.AddRecommendation("Сделайте title-тег более информативным (рекомендуется 30-60 символов)")
		} else if metaTitleLength > 60 {
			a.AddIssue(map[string]interface{}{
				"type":        "title_too_long",
				"severity":    "medium",
				"description": "Тег title слишком длинный",
				"current":     metaTitleLength,
				"recommended": "30-60 символов",
			})
			a.AddRecommendation("Сократите title-тег (рекомендуется максимум 60 символов для полного отображения в результатах поиска)")
		}

		// Проверка наличия ключевых слов в title
		if data.TextContent != "" && metaTitleLength > 0 {
			// Получаем потенциальные ключевые слова из контента
			words := strings.Fields(strings.ToLower(data.TextContent))
			wordCount := make(map[string]int)

			for _, word := range words {
				if len(word) > 3 && !isStopWord(word) {
					word = strings.Trim(word, ".,?!:;()")
					if word != "" {
						wordCount[word]++
					}
				}
			}

			// Определяем топ 3 ключевых слова
			type WordFreq struct {
				Word  string
				Count int
			}

			wordFreqs := []WordFreq{}
			for word, count := range wordCount {
				if count >= 3 {
					wordFreqs = append(wordFreqs, WordFreq{word, count})
				}
			}

			// Сортируем по частоте
			sort.Slice(wordFreqs, func(i, j int) bool {
				return wordFreqs[i].Count > wordFreqs[j].Count
			})

			// Берем топ-3 или меньше, если доступно меньше
			topKeywords := []string{}
			for i := 0; i < len(wordFreqs) && i < 3; i++ {
				topKeywords = append(topKeywords, wordFreqs[i].Word)
			}

			// Проверяем наличие в title
			keywordsInTitle := false
			titleLower := strings.ToLower(data.Title)

			for _, keyword := range topKeywords {
				if strings.Contains(titleLower, keyword) {
					keywordsInTitle = true
					break
				}
			}

			if !keywordsInTitle && len(topKeywords) > 0 {
				a.AddIssue(map[string]interface{}{
					"type":        "no_keywords_in_title",
					"severity":    "medium",
					"description": "В title отсутствуют ключевые слова из контента",
					"keywords":    topKeywords,
				})
				a.AddRecommendation("Добавьте в title основные ключевые слова из контента: " + strings.Join(topKeywords, ", "))
			}
		}
	}

	// Проверка мета-тега description
	metaDesc := ""
	if desc, ok := data.MetaTags["description"]; ok {
		metaDesc = desc
	}
	missingMetaDesc := metaDesc == ""
	metaDescLength := len(metaDesc)

	a.SetMetric("missing_meta_description", missingMetaDesc)
	a.SetMetric("meta_description_length", metaDescLength)

	// Анализ и рекомендации для description
	if missingMetaDesc {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_description",
			"severity":    "high",
			"description": "На странице отсутствует мета-тег description",
		})
		a.AddRecommendation("Добавьте мета-тег description с кратким описанием содержания страницы (рекомендуется 50-160 символов)")
	} else {
		// Проверка длины description
		if metaDescLength < 50 {
			a.AddIssue(map[string]interface{}{
				"type":        "description_too_short",
				"severity":    "medium",
				"description": "Мета-тег description слишком короткий",
				"current":     metaDescLength,
				"recommended": "50-160 символов",
			})
			a.AddRecommendation("Сделайте мета-тег description более информативным (рекомендуется 50-160 символов)")
		} else if metaDescLength > 160 {
			a.AddIssue(map[string]interface{}{
				"type":        "description_too_long",
				"severity":    "medium",
				"description": "Мета-тег description слишком длинный",
				"current":     metaDescLength,
				"recommended": "50-160 символов",
			})
			a.AddRecommendation("Сократите мета-тег description до 160 символов для оптимального отображения в результатах поиска")
		}
	}
}

// analyzeCanonical проверяет canonical URL
func (a *SEOAnalyzer) analyzeCanonical(data *parser.WebsiteData) {
	canonicalURL := ""
	for key, value := range data.MetaTags {
		if key == "canonical" {
			canonicalURL = value
			break
		}
	}
	a.SetMetric("canonical_url", canonicalURL)

	if canonicalURL == "" {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_canonical",
			"severity":    "medium",
			"description": "На странице отсутствует канонический URL",
		})
		a.AddRecommendation("Добавьте канонический URL для предотвращения проблем с дублированным контентом")
	} else {
		// Проверяем, является ли URL относительным
		isRelative := !strings.HasPrefix(canonicalURL, "http://") && !strings.HasPrefix(canonicalURL, "https://")

		if isRelative {
			a.AddIssue(map[string]interface{}{
				"type":        "relative_canonical",
				"severity":    "low",
				"description": "Канонический URL задан в относительном формате",
				"url":         canonicalURL,
			})
			a.AddRecommendation("Рекомендуется использовать абсолютный URL в каноническом теге для предотвращения потенциальных проблем")
		}

		// Проверяем соответствие текущему URL
		currentURL := data.URL
		if currentURL != "" && canonicalURL != "" &&
			!isRelative && currentURL != canonicalURL &&
			!strings.HasSuffix(currentURL, "/") && canonicalURL != currentURL+"/" {
			a.AddIssue(map[string]interface{}{
				"type":        "canonical_mismatch",
				"severity":    "medium",
				"description": "Канонический URL не соответствует URL страницы",
				"current":     currentURL,
				"canonical":   canonicalURL,
			})
			a.AddRecommendation("Убедитесь, что канонический URL правильно указывает на текущую страницу, если это основная версия контента")
		}
	}
}

// analyzeImages проверяет альтернативные тексты для изображений
func (a *SEOAnalyzer) analyzeImages(data *parser.WebsiteData) {
	// Проблемы с alt-текстами
	missingAlt := []map[string]string{}
	tooShortAlt := []map[string]string{}
	suspiciousAlt := []map[string]string{}

	for _, img := range data.Images {
		if img.Alt == "" {
			missingAlt = append(missingAlt, map[string]string{
				"url": img.URL,
				// "location": img.Location, // Removed as img.Location does not exist
			})
		} else if len(img.Alt) < 5 {
			tooShortAlt = append(tooShortAlt, map[string]string{
				"url": img.URL,
				"alt": img.Alt,
			})
		} else {
			// Проверяем на подозрительные alt-тексты (например, просто имя файла)
			fileName := filepath.Base(img.URL)
			fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

			if strings.Contains(img.Alt, fileNameWithoutExt) ||
				strings.Contains(strings.ToLower(img.Alt), "image") ||
				strings.Contains(strings.ToLower(img.Alt), "picture") {
				suspiciousAlt = append(suspiciousAlt, map[string]string{
					"url": img.URL,
					"alt": img.Alt,
				})
			}
		}
	}

	a.SetMetric("images_missing_alt", len(missingAlt))
	a.SetMetric("images_too_short_alt", len(tooShortAlt))
	a.SetMetric("images_suspicious_alt", len(suspiciousAlt))

	if len(missingAlt) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_alt",
			"severity":    "medium",
			"description": "Изображения без атрибута alt",
			"count":       len(missingAlt),
			"images":      missingAlt[:min(len(missingAlt), 5)], // Показываем до 5 примеров
		})
		a.AddRecommendation("Добавьте информативный alt-текст ко всем изображениям для улучшения доступности и SEO")
	}

	if len(tooShortAlt) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "too_short_alt",
			"severity":    "low",
			"description": "Изображения со слишком коротким атрибутом alt",
			"count":       len(tooShortAlt),
			"images":      tooShortAlt[:min(len(tooShortAlt), 5)], // Показываем до 5 примеров
		})
		a.AddRecommendation("Сделайте alt-текст более описательным (рекомендуется 5-125 символов)")
	}

	if len(suspiciousAlt) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "suspicious_alt",
			"severity":    "low",
			"description": "Изображения с подозрительным alt-текстом (содержит имя файла или общие слова)",
			"count":       len(suspiciousAlt),
			"images":      suspiciousAlt[:min(len(suspiciousAlt), 5)], // Показываем до 5 примеров
		})
		a.AddRecommendation("Сделайте alt-тексты более осмысленными и описательными, не используйте имя файла или общие слова, как 'image', 'picture'")
	}
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
