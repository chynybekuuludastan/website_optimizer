package analyzer

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// MobileAnalyzer анализирует мобильную адаптивность сайта
type MobileAnalyzer struct {
	*BaseAnalyzer
}

// NewMobileAnalyzer создает новый анализатор мобильной адаптивности
func NewMobileAnalyzer() *MobileAnalyzer {
	return &MobileAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
	}
}

// Analyze выполняет анализ мобильной адаптивности
func (a *MobileAnalyzer) Analyze(data *parser.WebsiteData) (map[string]interface{}, error) {
	// Анализ HTML-документа с помощью goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
	if err != nil {
		return a.GetMetrics(), err
	}

	// Проверка наличия метатега viewport
	a.analyzeViewport(doc)

	// Проверка использования медиа-запросов
	a.analyzeMediaQueries(data)

	// Проверка размера шрифта и читаемости
	a.analyzeFontSize(data)

	// Проверка размера элементов для сенсорного ввода
	a.analyzeTouchTargets(doc)

	// Проверка ширины контента
	a.analyzeContentWidth(doc)

	// Проверка использования фиксированных размеров
	a.analyzeFixedSizes(data)

	// Проверка оптимизации изображений
	a.analyzeImageOptimization(data)

	// Проверка наличия мобильных шаблонов
	a.analyzeMobileTemplates(data)

	// Расчет общей оценки
	score := a.CalculateScore()
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeViewport проверяет наличие и корректность метатега viewport
func (a *MobileAnalyzer) analyzeViewport(doc *goquery.Document) {
	viewportContent := ""

	doc.Find("meta[name='viewport']").Each(func(i int, s *goquery.Selection) {
		content, exists := s.Attr("content")
		if exists {
			viewportContent = content
		}
	})

	a.SetMetric("has_viewport", viewportContent != "")
	a.SetMetric("viewport_content", viewportContent)

	if viewportContent == "" {
		a.AddIssue(map[string]interface{}{
			"type":        "missing_viewport",
			"severity":    "high",
			"description": "Отсутствует метатег viewport",
		})
		a.AddRecommendation("Добавьте метатег viewport для правильного отображения на мобильных устройствах: <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	} else {
		// Проверка наличия необходимых параметров
		hasWidthDevice := strings.Contains(viewportContent, "width=device-width")
		hasInitialScale := strings.Contains(viewportContent, "initial-scale=1")

		if !hasWidthDevice || !hasInitialScale {
			a.AddIssue(map[string]interface{}{
				"type":        "incomplete_viewport",
				"severity":    "medium",
				"description": "Неполная конфигурация метатега viewport",
				"content":     viewportContent,
			})
			a.AddRecommendation("Убедитесь, что метатег viewport содержит 'width=device-width, initial-scale=1'")
		}
	}
}

// analyzeMediaQueries проверяет использование медиа-запросов
func (a *MobileAnalyzer) analyzeMediaQueries(data *parser.WebsiteData) {
	// Ищем медиа-запросы в HTML
	mediaQueryRegex := regexp.MustCompile(`@media\s*\([^)]+\)`)
	mediaQueries := mediaQueryRegex.FindAllString(data.HTML, -1)

	hasMediaQueries := len(mediaQueries) > 0
	a.SetMetric("has_media_queries", hasMediaQueries)
	a.SetMetric("media_queries_count", len(mediaQueries))

	if !hasMediaQueries {
		a.AddIssue(map[string]interface{}{
			"type":        "no_media_queries",
			"severity":    "high",
			"description": "Не обнаружены медиа-запросы для адаптивного дизайна",
		})
		a.AddRecommendation("Используйте CSS медиа-запросы для адаптации сайта под различные размеры экранов")
	}
}

// analyzeFontSize проверяет размер шрифта для мобильных устройств
func (a *MobileAnalyzer) analyzeFontSize(data *parser.WebsiteData) {
	// Ищем размеры шрифтов в px
	fontSizeRegex := regexp.MustCompile(`font-size: ?([0-9]{1,2})px`)
	matches := fontSizeRegex.FindAllStringSubmatch(data.HTML, -1)

	smallFonts := []string{}
	for _, match := range matches {
		if len(match) >= 2 {
			size := match[1]
			if size < "14" { // Минимальный рекомендуемый размер шрифта для мобильных
				smallFonts = append(smallFonts, size+"px")
			}
		}
	}

	a.SetMetric("small_mobile_fonts", smallFonts)

	if len(smallFonts) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "small_font_for_mobile",
			"severity":    "medium",
			"description": "Размер шрифта может быть слишком мал для мобильных устройств",
			"fonts":       smallFonts,
		})
		a.AddRecommendation("Используйте размер шрифта не менее 14px для основного текста на мобильных устройствах")
	}
}

// analyzeTouchTargets проверяет размер элементов для сенсорного ввода
func (a *MobileAnalyzer) analyzeTouchTargets(doc *goquery.Document) {
	smallTouchTargets := 0

	// Проверяем размеры кнопок и ссылок
	doc.Find("a, button, input[type='button'], input[type='submit'], input[type='checkbox'], input[type='radio']").Each(func(i int, s *goquery.Selection) {
		width, hasWidth := s.Attr("width")
		height, hasHeight := s.Attr("height")

		style, hasStyle := s.Attr("style")
		if hasStyle {
			// Простая проверка на наличие малых размеров в атрибуте style
			if strings.Contains(style, "width:") && (strings.Contains(style, "px") || strings.Contains(style, "em")) {
				widthRegex := regexp.MustCompile(`width: ?([0-9]{1,2})(px|em)`)
				widthMatches := widthRegex.FindStringSubmatch(style)
				if len(widthMatches) >= 3 {
					value := widthMatches[1]
					unit := widthMatches[2]
					if (unit == "px" && value < "44") || (unit == "em" && value < "2.75") {
						smallTouchTargets++
						return
					}
				}
			}

			if strings.Contains(style, "height:") && (strings.Contains(style, "px") || strings.Contains(style, "em")) {
				heightRegex := regexp.MustCompile(`height: ?([0-9]{1,2})(px|em)`)
				heightMatches := heightRegex.FindStringSubmatch(style)
				if len(heightMatches) >= 3 {
					value := heightMatches[1]
					unit := heightMatches[2]
					if (unit == "px" && value < "44") || (unit == "em" && value < "2.75") {
						smallTouchTargets++
						return
					}
				}
			}
		}

		// Проверяем значения атрибутов width и height
		if hasWidth && hasHeight {
			if width < "44" || height < "44" {
				smallTouchTargets++
			}
		}
	})

	a.SetMetric("small_touch_targets", smallTouchTargets)

	if smallTouchTargets > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "small_touch_targets",
			"severity":    "medium",
			"description": "Интерактивные элементы могут быть слишком малы для сенсорного ввода",
			"count":       smallTouchTargets,
		})
		a.AddRecommendation("Используйте интерактивные элементы размером не менее 44×44 пикселей для удобства сенсорного ввода")
	}
}

// analyzeContentWidth проверяет ширину контента
func (a *MobileAnalyzer) analyzeContentWidth(doc *goquery.Document) {
	fixedWidthElements := 0

	// Ищем элементы с фиксированной шириной, которая может вызвать горизонтальную прокрутку
	doc.Find("[width], [style*='width']").Each(func(i int, s *goquery.Selection) {
		width, hasWidthAttr := s.Attr("width")
		style, hasStyle := s.Attr("style")

		if hasWidthAttr {
			// Проверяем, что ширина указана в пикселях и больше типичной ширины мобильного экрана
			if strings.HasSuffix(width, "px") || (width >= "320" && !strings.Contains(width, "%")) {
				fixedWidthElements++
				return
			}
		}

		if hasStyle && strings.Contains(style, "width:") {
			if strings.Contains(style, "px") && !strings.Contains(style, "%") && !strings.Contains(style, "max-width") {
				widthRegex := regexp.MustCompile(`width: ?([0-9]{3,4})px`)
				widthMatches := widthRegex.FindStringSubmatch(style)
				if len(widthMatches) >= 2 {
					value := widthMatches[1]
					if value > "320" {
						fixedWidthElements++
						return
					}
				}
			}
		}
	})

	a.SetMetric("fixed_width_elements", fixedWidthElements)

	if fixedWidthElements > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "fixed_width_content",
			"severity":    "high",
			"description": "Контент с фиксированной шириной может вызвать горизонтальную прокрутку на мобильных устройствах",
			"count":       fixedWidthElements,
		})
		a.AddRecommendation("Используйте относительные единицы (%, em, rem) вместо фиксированных пикселей для ширины элементов")
	}
}

// analyzeFixedSizes проверяет использование фиксированных размеров
func (a *MobileAnalyzer) analyzeFixedSizes(data *parser.WebsiteData) {
	// Ищем CSS свойства с фиксированными размерами
	fixedWidthRegex := regexp.MustCompile(`width: ?[0-9]+px`)
	fixedHeightRegex := regexp.MustCompile(`height: ?[0-9]+px`)

	fixedWidths := fixedWidthRegex.FindAllString(data.HTML, -1)
	fixedHeights := fixedHeightRegex.FindAllString(data.HTML, -1)

	a.SetMetric("fixed_width_count", len(fixedWidths))
	a.SetMetric("fixed_height_count", len(fixedHeights))

	if len(fixedWidths) > 5 || len(fixedHeights) > 5 { // Порог в 5 фиксированных размеров
		a.AddIssue(map[string]interface{}{
			"type":         "too_many_fixed_sizes",
			"severity":     "medium",
			"description":  "Слишком много элементов с фиксированными размерами",
			"width_count":  len(fixedWidths),
			"height_count": len(fixedHeights),
		})
		a.AddRecommendation("Замените фиксированные размеры (px) на относительные (%, em, rem, vw, vh) для лучшей адаптивности")
	}
}

// analyzeImageOptimization проверяет оптимизацию изображений для мобильных устройств
func (a *MobileAnalyzer) analyzeImageOptimization(data *parser.WebsiteData) {
	// Проверяем, используются ли атрибуты srcset/sizes для изображений
	hasSrcset := strings.Contains(data.HTML, "srcset=")
	a.SetMetric("has_responsive_images", hasSrcset)

	totalImages := len(data.Images)
	largeImages := 0

	for _, img := range data.Images {
		if img.FileSize > 200000 { // 200KB - порог для изображений на мобильных
			largeImages++
		}
	}

	a.SetMetric("large_images_for_mobile", largeImages)

	if totalImages > 0 && !hasSrcset {
		a.AddIssue(map[string]interface{}{
			"type":        "no_responsive_images",
			"severity":    "medium",
			"description": "Изображения не используют атрибуты srcset/sizes для адаптивной загрузки",
		})
		a.AddRecommendation("Используйте атрибуты srcset и sizes для загрузки оптимизированных изображений на разных устройствах")
	}

	if largeImages > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "large_images_mobile",
			"severity":    "medium",
			"description": "Большие изображения могут замедлить загрузку на мобильных устройствах",
			"count":       largeImages,
		})
		a.AddRecommendation("Оптимизируйте размер изображений для мобильных устройств, используя сжатие и соответствующие форматы (WebP, AVIF)")
	}
}

// analyzeMobileTemplates проверяет наличие мобильных шаблонов
func (a *MobileAnalyzer) analyzeMobileTemplates(data *parser.WebsiteData) {
	// Проверяем наличие AMP или других мобильных шаблонов
	hasAMP := strings.Contains(data.HTML, "⚡") || strings.Contains(data.HTML, "amp-") || strings.Contains(data.HTML, "AMP")
	hasMobileTemplate := hasAMP

	// Проверяем каноническую ссылку на мобильную версию
	for key, value := range data.MetaTags {
		if key == "alternate" && strings.Contains(value, "mobile") {
			hasMobileTemplate = true
			break
		}
	}

	a.SetMetric("has_mobile_template", hasMobileTemplate)
	a.SetMetric("has_amp", hasAMP)

	if !hasMobileTemplate {
		// Это скорее информация, чем проблема, поэтому severity=low
		a.AddIssue(map[string]interface{}{
			"type":        "no_mobile_template",
			"severity":    "low",
			"description": "Нет специального мобильного шаблона (AMP и т.п.)",
		})
		a.AddRecommendation("Рассмотрите возможность реализации AMP (Accelerated Mobile Pages) для улучшения загрузки на мобильных устройствах")
	}
}
