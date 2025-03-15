package analyzer

import (
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
		BaseAnalyzer: NewBaseAnalyzer(),
	}
}

// Analyze выполняет SEO-анализ данных веб-сайта
func (a *SEOAnalyzer) Analyze(data *parser.WebsiteData) (map[string]interface{}, error) {
	// Проверка мета-тегов title и description
	a.analyzeTitleAndDescription(data)

	// Анализ структуры заголовков
	a.analyzeHeadings(data)

	// Проверка альтернативных текстов для изображений
	a.analyzeImages(data)

	// Анализ ссылок
	a.analyzeLinks(data)

	// Проверка canonical URL
	a.analyzeCanonical(data)

	// Анализ плотности ключевых слов
	a.analyzeKeywords(data)

	// Расчет общей оценки
	score := a.CalculateScore()
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeTitleAndDescription проверяет мета-теги title и description
func (a *SEOAnalyzer) analyzeTitleAndDescription(data *parser.WebsiteData) {
	// Проверка мета-тега title
	missingMetaTitle := data.Title == ""
	metaTitleLength := len(data.Title)

	a.SetMetric("missing_meta_title", missingMetaTitle)
	a.SetMetric("meta_title_length", metaTitleLength)

	if missingMetaTitle {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_title",
			"severity":    "high",
			"description": "На странице отсутствует тег title",
		})
		a.AddRecommendation("Добавьте информативный title-тег на страницу")
	} else if metaTitleLength < 30 || metaTitleLength > 60 {
		a.AddIssue(map[string]interface{}{
			"type":        "title_length",
			"severity":    "medium",
			"description": "Тег title либо слишком короткий, либо слишком длинный",
			"current":     metaTitleLength,
			"recommended": "30-60 символов",
		})
		if metaTitleLength < 30 {
			a.AddRecommendation("Сделайте title-тег более информативным (минимум 30 символов)")
		} else {
			a.AddRecommendation("Сократите title-тег (рекомендуется максимум 60 символов)")
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

	if missingMetaDesc {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_description",
			"severity":    "high",
			"description": "На странице отсутствует мета-тег description",
		})
		a.AddRecommendation("Добавьте мета-тег description с кратким описанием содержания страницы")
	} else if metaDescLength < 50 || metaDescLength > 160 {
		a.AddIssue(map[string]interface{}{
			"type":        "description_length",
			"severity":    "medium",
			"description": "Мета-тег description либо слишком короткий, либо слишком длинный",
			"current":     metaDescLength,
			"recommended": "50-160 символов",
		})
		if metaDescLength < 50 {
			a.AddRecommendation("Сделайте мета-тег description более информативным (минимум 50 символов)")
		} else {
			a.AddRecommendation("Сократите мета-тег description (рекомендуется максимум 160 символов)")
		}
	}
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

// analyzeImages проверяет альтернативные тексты для изображений
func (a *SEOAnalyzer) analyzeImages(data *parser.WebsiteData) {
	missingAltTags := []string{}
	for _, img := range data.Images {
		if img.Alt == "" {
			missingAltTags = append(missingAltTags, img.URL)
		}
	}
	a.SetMetric("missing_alt_tags", missingAltTags)

	if len(missingAltTags) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_alt",
			"severity":    "medium",
			"description": "Изображения без атрибута alt",
			"count":       len(missingAltTags),
		})
		a.AddRecommendation("Добавьте информативный alt-текст ко всем изображениям")
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
			"severity":    "low",
			"description": "На странице отсутствует канонический URL",
		})
		a.AddRecommendation("Добавьте канонический URL для предотвращения проблем с дублированным контентом")
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
