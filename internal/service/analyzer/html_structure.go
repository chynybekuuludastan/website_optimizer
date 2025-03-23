package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// StructureAnalyzer анализирует HTML-структуру сайта
type StructureAnalyzer struct {
	*BaseAnalyzer
}

// NewStructureAnalyzer создает новый анализатор структуры
func NewStructureAnalyzer() *StructureAnalyzer {
	return &StructureAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(StructureType),
	}
}

// Analyze выполняет анализ структуры HTML
func (a *StructureAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Проверка контекста
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Проверяем, присутствуют ли уже результаты Lighthouse
	skipFullAnalysis := false

	if lighthouseResults, ok := prevResults[LighthouseType]; ok {
		// Определяем аудиты, связанные со структурой HTML
		structureAuditTypes := map[string]bool{
			"document-title":         true,
			"html-has-lang":          true,
			"meta-description":       true,
			"heading-order":          true,
			"duplicate-id-active":    true,
			"duplicate-id-aria":      true,
			"duplicate-id":           true,
			"table-duplicate-name":   true,
			"td-headers-attr":        true,
			"th-has-data-cells":      true,
			"valid-lang":             true,
			"html-xml-lang-mismatch": true,
		}

		// Извлекаем данные аудитов структуры
		structureAudits := extractAuditsData(lighthouseResults, structureAuditTypes)

		if len(structureAudits) > 0 {
			a.SetMetric("lighthouse_structure_audits", structureAudits)

			// Получаем проблемы из аудитов
			structureIssues := getAuditIssues(structureAudits, 0.9)

			// Добавляем проблемы и рекомендации
			for _, issue := range structureIssues {
				a.AddIssue(issue)
				if description, ok := issue["details"].(string); ok {
					a.AddRecommendation(description)
				}
			}

			// Если есть достаточно данных от Lighthouse, можем пропустить часть анализа
			if len(structureAudits) > 3 {
				skipFullAnalysis = true
			}
		}
	}

	// Анализ HTML-структуры с помощью goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
	if err != nil {
		a.AddIssue(map[string]interface{}{
			"type":        "invalid_html",
			"severity":    "high",
			"description": "HTML не может быть правильно проанализирован",
			"error":       err.Error(),
		})
		a.SetMetric("valid_html", false)
		return a.GetMetrics(), err
	}

	a.SetMetric("valid_html", true)

	// Проверка наличия DOCTYPE
	a.analyzeDoctype(data.HTML)

	// Если результаты Lighthouse недостаточны или их нет, выполняем полный анализ
	if !skipFullAnalysis {
		a.analyzeSemanticTags(doc)
		a.analyzeHeadingStructure(doc)
		a.analyzeImagesAlt(doc)
		a.analyzeFormAccessibility(doc)
		a.analyzeDuplicateIds(doc)
		a.analyzeListStructure(doc)
		a.analyzeTableStructure(doc)
	}

	// Расчет общей оценки
	score := a.CalculateScore()
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeDoctype проверяет наличие DOCTYPE
func (a *StructureAnalyzer) analyzeDoctype(html string) {
	hasDoctype := strings.Contains(strings.ToLower(html), "<!doctype html")
	a.SetMetric("has_doctype", hasDoctype)

	if !hasDoctype {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_doctype",
			"severity":    "high",
			"description": "Отсутствует объявление DOCTYPE",
		})
		a.AddRecommendation("Добавьте объявление DOCTYPE в начало HTML-документа")
	}
}

// analyzeSemanticTags проверяет использование семантических тегов
func (a *StructureAnalyzer) analyzeSemanticTags(doc *goquery.Document) {
	// Основные семантические теги и их важность
	criticalTags := map[string]bool{
		"header": true,
		"main":   true,
		"footer": true,
	}

	semanticTags := map[string]int{
		"header":     doc.Find("header").Length(),
		"footer":     doc.Find("footer").Length(),
		"nav":        doc.Find("nav").Length(),
		"main":       doc.Find("main").Length(),
		"article":    doc.Find("article").Length(),
		"section":    doc.Find("section").Length(),
		"aside":      doc.Find("aside").Length(),
		"figure":     doc.Find("figure").Length(),
		"figcaption": doc.Find("figcaption").Length(),
		"time":       doc.Find("time").Length(),
	}

	a.SetMetric("semantic_tags", semanticTags)

	// Подсчитываем количество используемых семантических тегов
	usedSemanticTags := 0
	usedCriticalTags := 0
	for tag, count := range semanticTags {
		if count > 0 {
			usedSemanticTags++
			if criticalTags[tag] {
				usedCriticalTags++
			}
		}
	}

	// Проверяем использование важных семантических тегов
	missingCriticalTags := []string{}
	for tag := range criticalTags {
		if semanticTags[tag] == 0 {
			missingCriticalTags = append(missingCriticalTags, tag)
		}
	}

	// Проверяем вложенность семантических тегов
	nestedSemanticIssues := 0
	doc.Find("section").Each(func(i int, s *goquery.Selection) {
		if s.Find("h1, h2, h3, h4, h5, h6").Length() == 0 {
			nestedSemanticIssues++
		}
	})

	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		if s.Find("h1, h2, h3, h4, h5, h6").Length() == 0 {
			nestedSemanticIssues++
		}
	})

	a.SetMetric("nested_semantic_issues", nestedSemanticIssues)

	if len(missingCriticalTags) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":         "missing_critical_semantic_tags",
			"severity":     "high",
			"description":  "Отсутствуют критически важные семантические элементы",
			"missing_tags": missingCriticalTags,
		})
		a.AddRecommendation("Добавьте основные семантические элементы: " + strings.Join(missingCriticalTags, ", "))
	}

	if usedSemanticTags < 4 { // Требуем хотя бы 4 разных семантических элемента
		a.AddIssue(map[string]interface{}{
			"type":        "insufficient_semantic_html",
			"severity":    "medium",
			"description": "Недостаточное использование семантических HTML-элементов",
			"used_tags":   usedSemanticTags,
		})
		a.AddRecommendation("Используйте больше семантических HTML-элементов (header, nav, main, article, section, aside, figure, time и т.д.)")
	}

	if nestedSemanticIssues > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "semantic_nesting_issues",
			"severity":    "medium",
			"description": "Элементы section и article должны содержать заголовки",
			"count":       nestedSemanticIssues,
		})
		a.AddRecommendation("Добавьте заголовки (h1-h6) в элементы section и article для улучшения структуры")
	}
}

// analyzeHeadingStructure проверяет иерархию заголовков
func (a *StructureAnalyzer) analyzeHeadingStructure(doc *goquery.Document) {
	headingLevels := map[string]int{
		"h1": doc.Find("h1").Length(),
		"h2": doc.Find("h2").Length(),
		"h3": doc.Find("h3").Length(),
		"h4": doc.Find("h4").Length(),
		"h5": doc.Find("h5").Length(),
		"h6": doc.Find("h6").Length(),
	}

	a.SetMetric("heading_levels", headingLevels)

	// Собираем содержимое заголовков для проверки дубликатов
	headingContent := make(map[string][]string)
	for level := 1; level <= 6; level++ {
		headingTag := fmt.Sprintf("h%d", level)
		content := []string{}

		doc.Find(headingTag).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			content = append(content, text)
		})

		headingContent[headingTag] = content
	}

	// Проверка наличия H1
	if headingLevels["h1"] == 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_h1",
			"severity":    "high",
			"description": "Отсутствует заголовок H1",
		})
		a.AddRecommendation("Добавьте заголовок H1, который четко описывает основное содержание страницы")
	} else if headingLevels["h1"] > 1 {
		// Проверка дубликатов H1
		h1Texts := headingContent["h1"]
		duplicateH1 := map[string]int{}

		for _, text := range h1Texts {
			duplicateH1[text]++
		}

		duplicates := []string{}
		for text, count := range duplicateH1 {
			if count > 1 {
				duplicates = append(duplicates, text)
			}
		}

		if len(duplicates) > 0 {
			a.AddIssue(map[string]interface{}{
				"type":        "duplicate_h1_content",
				"severity":    "high",
				"description": "На странице есть дублирующиеся заголовки H1",
				"duplicates":  duplicates,
			})
			a.AddRecommendation("Убедитесь, что каждый заголовок H1 уникален и описывает основное содержание страницы")
		} else {
			a.AddIssue(map[string]interface{}{
				"type":        "multiple_h1",
				"severity":    "medium",
				"description": "На странице несколько заголовков H1",
				"count":       headingLevels["h1"],
			})
			a.AddRecommendation("Используйте только один заголовок H1 на странице")
		}
	}

	// Проверка всей иерархии заголовков
	headingsInOrder := true
	skippedLevels := []string{}

	// Проверяем порядок заголовков
	if headingLevels["h1"] == 0 && (headingLevels["h2"] > 0 || headingLevels["h3"] > 0 ||
		headingLevels["h4"] > 0 || headingLevels["h5"] > 0 ||
		headingLevels["h6"] > 0) {
		headingsInOrder = false
		a.AddIssue(map[string]interface{}{
			"type":        "heading_order",
			"severity":    "medium",
			"description": "Заголовки используются без H1",
		})
		a.AddRecommendation("Начните иерархию заголовков с H1, затем используйте H2, H3 и т.д.")
	}

	// Проверка пропущенных уровней заголовков
	for i := 1; i < 6; i++ {
		currentTag := fmt.Sprintf("h%d", i)
		nextTag := fmt.Sprintf("h%d", i+2)

		if headingLevels[currentTag] > 0 && headingLevels[nextTag] > 0 && headingLevels[fmt.Sprintf("h%d", i+1)] == 0 {
			headingsInOrder = false
			skippedLevels = append(skippedLevels, fmt.Sprintf("h%d -> h%d", i, i+2))
		}
	}

	if len(skippedLevels) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "skipped_heading_levels",
			"severity":    "medium",
			"description": "Пропущены уровни в иерархии заголовков",
			"skipped":     skippedLevels,
		})
		a.AddRecommendation("Не пропускайте уровни в иерархии заголовков. Используйте последовательную структуру (H1 -> H2 -> H3 и т.д.)")
	}

	// Проверка очень длинных заголовков
	longHeadings := []string{}
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) > 70 {
			tag := goquery.NodeName(s)
			longHeadings = append(longHeadings, fmt.Sprintf("%s (%d символов)", tag, len(text)))
		}
	})

	if len(longHeadings) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "long_headings",
			"severity":    "low",
			"description": "Слишком длинные заголовки на странице",
			"headings":    longHeadings,
		})
		a.AddRecommendation("Сократите длинные заголовки для улучшения читаемости и SEO (рекомендуется до 70 символов)")
	}

	a.SetMetric("headings_in_order", headingsInOrder)
}

// analyzeImagesAlt проверяет наличие alt-атрибутов для изображений
func (a *StructureAnalyzer) analyzeImagesAlt(doc *goquery.Document) {
	totalImages := doc.Find("img").Length()
	missingAlt := 0

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		alt, exists := s.Attr("alt")
		if !exists || alt == "" {
			missingAlt++
		}
	})

	a.SetMetric("total_images", totalImages)
	a.SetMetric("images_missing_alt", missingAlt)

	if missingAlt > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_alt",
			"severity":    "medium",
			"description": "Изображения без атрибута alt",
			"count":       missingAlt,
			"total":       totalImages,
		})
		a.AddRecommendation("Добавьте атрибут alt ко всем изображениям для улучшения доступности и SEO")
	}
}

// analyzeFormAccessibility проверяет доступность форм
func (a *StructureAnalyzer) analyzeFormAccessibility(doc *goquery.Document) {
	forms := doc.Find("form").Length()
	formsWithLabels := 0

	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		inputs := s.Find("input, select, textarea").Length()
		labels := s.Find("label").Length()

		if labels >= inputs {
			formsWithLabels++
		}
	})

	a.SetMetric("total_forms", forms)
	a.SetMetric("forms_with_labels", formsWithLabels)

	if forms > 0 && formsWithLabels < forms {
		a.AddIssue(map[string]interface{}{
			"type":        "forms_without_labels",
			"severity":    "medium",
			"description": "Формы без достаточного количества меток (labels)",
			"count":       forms - formsWithLabels,
			"total":       forms,
		})
		a.AddRecommendation("Добавьте метки (label) для всех полей форм для улучшения доступности")
	}
}

// analyzeDuplicateIds проверяет наличие дублированных идентификаторов
func (a *StructureAnalyzer) analyzeDuplicateIds(doc *goquery.Document) {
	ids := make(map[string]int)

	doc.Find("[id]").Each(func(i int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		ids[id]++
	})

	duplicatedIds := make([]string, 0)
	for id, count := range ids {
		if count > 1 {
			duplicatedIds = append(duplicatedIds, id)
		}
	}

	a.SetMetric("duplicated_ids", duplicatedIds)

	if len(duplicatedIds) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "duplicated_ids",
			"severity":    "high",
			"description": "На странице есть дублированные идентификаторы (id)",
			"count":       len(duplicatedIds),
			"ids":         duplicatedIds,
		})
		a.AddRecommendation("Удалите дублированные идентификаторы. ID должны быть уникальными в пределах документа")
	}
}

// analyzeListStructure проверяет структуру списков
func (a *StructureAnalyzer) analyzeListStructure(doc *goquery.Document) {
	invalidLists := 0

	// Проверяем, что внутри ul и ol есть только li
	doc.Find("ul, ol").Each(func(i int, s *goquery.Selection) {
		s.Children().Each(func(j int, child *goquery.Selection) {
			if child.Get(0).Data != "li" {
				invalidLists++
			}
		})
	})

	a.SetMetric("invalid_lists", invalidLists)

	if invalidLists > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "invalid_list_structure",
			"severity":    "medium",
			"description": "Неправильная структура списков (ul/ol должны содержать только li)",
			"count":       invalidLists,
		})
		a.AddRecommendation("Убедитесь, что списки (ul/ol) содержат только элементы li")
	}
}

// analyzeTableStructure проверяет структуру таблиц
func (a *StructureAnalyzer) analyzeTableStructure(doc *goquery.Document) {
	tables := doc.Find("table").Length()
	tablesWithHeaders := doc.Find("table th, table thead").Length()

	a.SetMetric("total_tables", tables)
	a.SetMetric("tables_with_headers", tablesWithHeaders)

	if tables > 0 && tablesWithHeaders == 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "tables_without_headers",
			"severity":    "medium",
			"description": "Таблицы без заголовков",
			"count":       tables,
		})
		a.AddRecommendation("Добавьте заголовки (th) или thead к таблицам для улучшения доступности и структуры")
	}
}
