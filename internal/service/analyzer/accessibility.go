package analyzer

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AccessibilityAnalyzer анализирует доступность сайта согласно WCAG
type AccessibilityAnalyzer struct {
	*BaseAnalyzer
}

// NewAccessibilityAnalyzer создает новый анализатор доступности
func NewAccessibilityAnalyzer() *AccessibilityAnalyzer {
	return &AccessibilityAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
	}
}

// Analyze выполняет анализ доступности сайта
func (a *AccessibilityAnalyzer) Analyze(data *parser.WebsiteData) (map[string]interface{}, error) {
	// Анализ с помощью goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
	if err != nil {
		return a.GetMetrics(), err
	}

	// Проверка изображений без alt-текста
	a.analyzeMissingAltText(data, doc)

	// Проверка форм с отсутствующими метками
	a.analyzeMissingLabels(doc)

	// Проверка потенциальных проблем с контрастностью
	a.analyzeContrastIssues(data)

	// Проверка использования ARIA-атрибутов
	a.analyzeAriaAttributes(doc)

	// Проверка использования семантического HTML
	a.analyzeSemanticHTML(doc)

	// Проверка наличия skip-ссылок для клавиатурной навигации
	a.analyzeSkipLinks(doc)

	// Проверка наличия проблем с tabindex
	a.analyzeTabindex(doc)

	// Проверка доступности форм
	a.analyzeFormsAccessibility(doc)

	// Проверка размера шрифта
	a.analyzeFontSize(data)

	// Проверка языка страницы
	a.analyzeLanguage(doc)

	// Расчет общей оценки
	score := a.CalculateScore()
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeMissingAltText проверяет наличие alt-текста у изображений
func (a *AccessibilityAnalyzer) analyzeMissingAltText(data *parser.WebsiteData, doc *goquery.Document) {
	missingAltText := []string{}

	for _, img := range data.Images {
		if img.Alt == "" {
			missingAltText = append(missingAltText, img.URL)
		}
	}

	// Дополнительная проверка с помощью goquery
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		alt, exists := s.Attr("alt")
		src, _ := s.Attr("src")
		if !exists || alt == "" {
			// Проверяем, что URL еще не в списке
			alreadyAdded := false
			for _, url := range missingAltText {
				if url == src {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				missingAltText = append(missingAltText, src)
			}
		}
	})

	a.SetMetric("missing_alt_text", missingAltText)

	if len(missingAltText) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_alt_text",
			"severity":    "high",
			"description": "Изображения без альтернативного текста",
			"count":       len(missingAltText),
		})
		a.AddRecommendation("Добавьте информативный alt-текст ко всем изображениям для людей с нарушениями зрения")
	}
}

// analyzeMissingLabels проверяет формы на наличие меток
func (a *AccessibilityAnalyzer) analyzeMissingLabels(doc *goquery.Document) {
	formFields := doc.Find("input, select, textarea").Length()
	labeledFields := doc.Find("label[for], input[aria-label], select[aria-label], textarea[aria-label]").Length()
	missingLabelForms := formFields - labeledFields

	a.SetMetric("missing_label_forms", missingLabelForms)

	if missingLabelForms > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_form_labels",
			"severity":    "high",
			"description": "Поля форм без соответствующих меток",
			"count":       missingLabelForms,
		})
		a.AddRecommendation("Добавьте явные метки (label) ко всем полям форм")
	}
}

// analyzeContrastIssues проверяет потенциальные проблемы с контрастностью
func (a *AccessibilityAnalyzer) analyzeContrastIssues(data *parser.WebsiteData) {
	// Простая проверка на потенциальные проблемы с контрастностью (эвристика на основе ключевых слов)
	lowContrastColors := []string{"white", "#fff", "#ffffff", "yellow", "#ffff00", "lightgray", "lightgrey", "#d3d3d3"}
	contrastIssues := []map[string]interface{}{}

	for _, color := range lowContrastColors {
		if strings.Contains(data.HTML, "color:"+color) || strings.Contains(data.HTML, "color: "+color) {
			contrastIssues = append(contrastIssues, map[string]interface{}{
				"type":  "potential_low_contrast",
				"color": color,
			})
		}
	}

	a.SetMetric("contrast_issues", contrastIssues)

	if len(contrastIssues) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "potential_contrast_issues",
			"severity":    "medium",
			"description": "Потенциальные проблемы с контрастностью текста",
			"count":       len(contrastIssues),
		})
		a.AddRecommendation("Обеспечьте достаточный контраст цветов для всего текста (минимум 4.5:1 для обычного текста, 3:1 для крупного текста)")
	}
}

// analyzeAriaAttributes проверяет использование ARIA-атрибутов
func (a *AccessibilityAnalyzer) analyzeAriaAttributes(doc *goquery.Document) {
	ariaAttributesUsed := doc.Find("[aria-*]").Length() > 0
	a.SetMetric("aria_attributes_used", ariaAttributesUsed)

	if !ariaAttributesUsed {
		a.AddIssue(map[string]interface{}{
			"type":        "no_aria",
			"severity":    "medium",
			"description": "ARIA-атрибуты не используются для вспомогательных технологий",
		})
		a.AddRecommendation("Используйте ARIA-атрибуты для улучшения доступности для вспомогательных технологий")
	}
}

// analyzeSemanticHTML проверяет использование семантического HTML
func (a *AccessibilityAnalyzer) analyzeSemanticHTML(doc *goquery.Document) {
	semanticTags := []string{"header", "footer", "nav", "main", "article", "section", "aside"}
	semanticFound := 0

	for _, tag := range semanticTags {
		count := doc.Find(tag).Length()
		if count > 0 {
			semanticFound++
		}
	}

	semanticHTMLUsed := semanticFound >= 3 // Требуем хотя бы 3 различных семантических элемента
	a.SetMetric("semantic_html_used", semanticHTMLUsed)

	if !semanticHTMLUsed {
		a.AddIssue(map[string]interface{}{
			"type":        "insufficient_semantic_html",
			"severity":    "medium",
			"description": "Недостаточное использование семантических HTML-элементов",
		})
		a.AddRecommendation("Используйте больше семантических HTML-элементов (header, nav, main и т.д.)")
	}
}

// analyzeSkipLinks проверяет наличие skip-ссылок для клавиатурной навигации
func (a *AccessibilityAnalyzer) analyzeSkipLinks(doc *goquery.Document) {
	hasSkipLinks := doc.Find("a[href='#content'], a[href='#main']").Length() > 0
	a.SetMetric("has_skip_links", hasSkipLinks)

	if !hasSkipLinks {
		a.AddIssue(map[string]interface{}{
			"type":        "no_skip_links",
			"severity":    "medium",
			"description": "Отсутствуют skip-ссылки для клавиатурной навигации",
		})
		a.AddRecommendation("Добавьте skip-ссылки для обхода навигации для пользователей клавиатуры")
	}
}

// analyzeTabindex проверяет использование tabindex
func (a *AccessibilityAnalyzer) analyzeTabindex(doc *goquery.Document) {
	tabindexIssues := []string{}

	doc.Find("[tabindex]").Each(func(i int, s *goquery.Selection) {
		tabindex, exists := s.Attr("tabindex")
		if exists && tabindex != "0" && tabindex != "-1" {
			tabindexIssues = append(tabindexIssues, s.Text())
		}
	})

	a.SetMetric("tabindex_issues", tabindexIssues)

	if len(tabindexIssues) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "tabindex_issue",
			"severity":    "medium",
			"description": "Избегайте использования значений tabindex больше 0",
			"count":       len(tabindexIssues),
		})
		a.AddRecommendation("Используйте значения tabindex только 0 или -1")
	}
}

// analyzeFormsAccessibility проверяет доступность форм
func (a *AccessibilityAnalyzer) analyzeFormsAccessibility(doc *goquery.Document) {
	formWithRequired := doc.Find("form input[required], form [aria-required='true']").Length()
	forms := doc.Find("form").Length()

	accessibleForms := formWithRequired > 0 || forms == 0
	a.SetMetric("accessible_forms", accessibleForms)

	if !accessibleForms && forms > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "inaccessible_forms",
			"severity":    "medium",
			"description": "Формы должны указывать обязательные поля",
		})
		a.AddRecommendation("Отмечайте обязательные поля формы с помощью атрибута required или aria-required")
	}
}

// analyzeFontSize проверяет размер шрифта
func (a *AccessibilityAnalyzer) analyzeFontSize(data *parser.WebsiteData) {
	smallFontRegex := regexp.MustCompile(`font-size: ?([0-9]{1,2})(px|pt);`)
	matches := smallFontRegex.FindAllStringSubmatch(data.HTML, -1)

	hasSmallFont := false
	smallFonts := []map[string]string{}

	for _, match := range matches {
		if len(match) >= 3 {
			size := match[1]
			unit := match[2]

			if (unit == "px" && size < "12") || (unit == "pt" && size < "9") {
				hasSmallFont = true
				smallFonts = append(smallFonts, map[string]string{
					"size": size,
					"unit": unit,
				})
			}
		}
	}

	a.SetMetric("small_fonts", smallFonts)

	if hasSmallFont {
		a.AddIssue(map[string]interface{}{
			"type":        "small_font_size",
			"severity":    "medium",
			"description": "Размер шрифта может быть слишком мал для удобочитаемости",
			"fonts":       smallFonts,
		})
		a.AddRecommendation("Используйте размер шрифта не менее 12px или 9pt для удобочитаемости")
	}
}

// analyzeLanguage проверяет указание языка страницы
func (a *AccessibilityAnalyzer) analyzeLanguage(doc *goquery.Document) {
	html := doc.Find("html")
	lang, exists := html.Attr("lang")

	a.SetMetric("has_language", exists)
	if exists {
		a.SetMetric("language", lang)
	}

	if !exists {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_language",
			"severity":    "medium",
			"description": "Не указан язык страницы",
		})
		a.AddRecommendation("Укажите атрибут lang на элементе html для обозначения языка страницы")
	}
}
