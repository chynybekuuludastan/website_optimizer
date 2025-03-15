package analyzer

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// ContentAnalyzer анализирует качество и читаемость контента
type ContentAnalyzer struct {
	*BaseAnalyzer
}

// NewContentAnalyzer создает новый анализатор контента
func NewContentAnalyzer() *ContentAnalyzer {
	return &ContentAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(ContentType),
	}
}

// Analyze выполняет анализ контента
func (a *ContentAnalyzer) Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	// Проверка контекста
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Извлекаем чистый текст
	text := data.TextContent

	// Если текст пустой, нет смысла анализировать
	if text == "" {
		a.AddIssue(map[string]interface{}{
			"type":        "no_text_content",
			"severity":    "high",
			"description": "Не найден текстовый контент",
		})
		a.SetMetric("has_content", false)
		return a.GetMetrics(), nil
	}

	a.SetMetric("has_content", true)

	// Проверяем, доступны ли результаты Lighthouse
	lighthouseUsed := false

	if lighthouseResults, ok := prevResults[LighthouseType]; ok {
		// Определяем аудиты, связанные с контентом
		contentAuditTypes := map[string]bool{
			"document-title":   true,
			"meta-description": true,
			"link-text":        true,
			"hreflang":         true,
			"plugins":          true,
			"canonical":        true,
			"structured-data":  true,
			"font-size":        true,
			"heading-order":    true,
		}

		// Извлекаем данные аудитов контента
		contentAudits := extractAuditsData(lighthouseResults, contentAuditTypes)

		if len(contentAudits) > 0 {
			a.SetMetric("lighthouse_content_audits", contentAudits)
			lighthouseUsed = true

			// Получаем проблемы из аудитов
			contentIssues := getAuditIssues(contentAudits, 0.9)

			// Добавляем проблемы и рекомендации
			for _, issue := range contentIssues {
				a.AddIssue(issue)
				if description, ok := issue["details"].(string); ok {
					a.AddRecommendation(description)
				}
			}
		}
	}

	// Анализ базовых метрик текста (всегда проводим этот анализ)
	a.analyzeBasicMetrics(text)

	// Для более глубокого анализа, который не охватывается Lighthouse
	// Анализ читаемости
	a.analyzeReadability(text)

	// Проверка плотности ключевых слов
	a.analyzeKeywordDensity(text)

	// Дополнительные проверки, если у нас нет данных из Lighthouse
	// или если мы хотим дополнить анализ Lighthouse своими проверками
	if !lighthouseUsed {
		a.analyzeHeadingLength(data)
		a.analyzeParagraphStructure(data.HTML)
		a.analyzeDuplicateContent(data.HTML)
		a.analyzeTextToHtmlRatio(text, data.HTML)
	}

	// Расчет общей оценки
	score := a.CalculateScore()
	a.SetMetric("score", score)

	return a.GetMetrics(), nil
}

// analyzeBasicMetrics анализирует базовые метрики текста
func (a *ContentAnalyzer) analyzeBasicMetrics(text string) {
	// Количество слов
	words := strings.Fields(text)
	wordCount := len(words)

	// Количество символов
	charCount := len(text)

	// Среднее количество слов в предложении
	sentences := splitIntoSentences(text)
	sentenceCount := len(sentences)

	var avgWordsPerSentence float64
	if sentenceCount > 0 {
		avgWordsPerSentence = float64(wordCount) / float64(sentenceCount)
	}

	// Среднее количество символов в слове
	var avgCharsPerWord float64
	if wordCount > 0 {
		avgCharsPerWord = float64(charCount) / float64(wordCount)
	}

	a.SetMetric("word_count", wordCount)
	a.SetMetric("character_count", charCount)
	a.SetMetric("sentence_count", sentenceCount)
	a.SetMetric("avg_words_per_sentence", avgWordsPerSentence)
	a.SetMetric("avg_chars_per_word", avgCharsPerWord)

	// Проверка длины контента
	if wordCount < 300 {
		a.AddIssue(map[string]interface{}{
			"type":        "low_word_count",
			"severity":    "medium",
			"description": "Недостаточное количество слов для качественного контента",
			"word_count":  wordCount,
			"recommended": "минимум 300 слов",
		})
		a.AddRecommendation("Увеличьте объем текстового контента до 300+ слов для лучшей индексации и полезности для пользователей")
	}

	// Проверка длины предложений
	if avgWordsPerSentence > 25 {
		a.AddIssue(map[string]interface{}{
			"type":        "long_sentences",
			"severity":    "medium",
			"description": "Предложения слишком длинные, что затрудняет чтение",
			"avg_words":   avgWordsPerSentence,
			"recommended": "15-20 слов",
		})
		a.AddRecommendation("Сократите длину предложений для улучшения читаемости. Оптимальная длина - 15-20 слов")
	}
}

// analyzeReadability анализирует читаемость текста
func (a *ContentAnalyzer) analyzeReadability(text string) {
	// Расчет индекса Флеша-Кинкейда (для русского языка - адаптированная версия)
	// Формула: 206.835 - 1.015 * (words / sentences) - 84.6 * (syllables / words)

	words := strings.Fields(text)
	wordCount := len(words)
	sentences := splitIntoSentences(text)
	sentenceCount := max(1, len(sentences))

	syllables := countSyllables(text)

	var fleschScore float64
	if wordCount > 0 {
		fleschScore = 206.835 - 1.015*(float64(wordCount)/float64(sentenceCount)) - 84.6*(float64(syllables)/float64(wordCount))
	}

	// Нормализация оценки (в оригинале FRE: 0-100, где выше - проще)
	if fleschScore < 0 {
		fleschScore = 0
	} else if fleschScore > 100 {
		fleschScore = 100
	}

	a.SetMetric("flesch_reading_ease", fleschScore)
	a.SetMetric("syllable_count", syllables)

	var readabilityLevel string
	if fleschScore >= 90 {
		readabilityLevel = "Очень легко читается (5 класс)"
	} else if fleschScore >= 80 {
		readabilityLevel = "Легко читается (6 класс)"
	} else if fleschScore >= 70 {
		readabilityLevel = "Довольно легко читается (7 класс)"
	} else if fleschScore >= 60 {
		readabilityLevel = "Стандартная сложность (8-9 класс)"
	} else if fleschScore >= 50 {
		readabilityLevel = "Умеренно сложно (10-11 класс)"
	} else if fleschScore >= 30 {
		readabilityLevel = "Сложно (студент)"
	} else {
		readabilityLevel = "Очень сложно (выпускник вуза)"
	}

	a.SetMetric("readability_level", readabilityLevel)

	if fleschScore < 50 {
		a.AddIssue(map[string]interface{}{
			"type":        "complex_readability",
			"severity":    "medium",
			"description": "Текст может быть слишком сложным для понимания",
			"score":       fleschScore,
			"level":       readabilityLevel,
		})
		a.AddRecommendation("Упростите текст, используйте более короткие предложения и слова для улучшения читаемости")
	}
}

// analyzeKeywordDensity анализирует плотность ключевых слов
func (a *ContentAnalyzer) analyzeKeywordDensity(text string) {
	words := strings.Fields(strings.ToLower(text))
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
	highDensityKeywords := []string{}

	for word, count := range wordCount {
		if count >= 3 { // Учитываем только слова, которые появляются не менее 3 раз
			density := float64(count) / float64(totalWords) * 100
			keywordDensity[word] = density

			if density > 5.0 {
				highDensityKeywords = append(highDensityKeywords, word)
			}
		}
	}

	a.SetMetric("keyword_density", keywordDensity)

	if len(highDensityKeywords) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "keyword_stuffing",
			"severity":    "medium",
			"description": "Слишком высокая плотность ключевых слов",
			"keywords":    highDensityKeywords,
		})
		a.AddRecommendation("Уменьшите плотность ключевых слов для более естественного текста. Оптимальная плотность - 1-3%")
	}
}

// analyzeHeadingLength анализирует длину заголовков
func (a *ContentAnalyzer) analyzeHeadingLength(data *parser.WebsiteData) {
	longHeadings := []string{}

	// Проверяем длину заголовков H1
	for _, heading := range data.H1 {
		words := strings.Fields(heading)
		if len(words) > 10 {
			longHeadings = append(longHeadings, "H1: "+heading)
		}
	}

	// Проверяем длину заголовков H2
	for _, heading := range data.H2 {
		words := strings.Fields(heading)
		if len(words) > 13 {
			longHeadings = append(longHeadings, "H2: "+heading)
		}
	}

	a.SetMetric("long_headings", longHeadings)

	if len(longHeadings) > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "long_headings",
			"severity":    "low",
			"description": "Некоторые заголовки слишком длинные",
			"count":       len(longHeadings),
		})
		a.AddRecommendation("Сократите длинные заголовки для улучшения читаемости. Оптимальная длина - 5-7 слов")
	}
}

// analyzeParagraphStructure анализирует структуру параграфов
func (a *ContentAnalyzer) analyzeParagraphStructure(html string) {
	// Находим все параграфы
	paragraphRegex := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
	paragraphs := paragraphRegex.FindAllString(html, -1)

	longParagraphs := 0
	shortParagraphs := 0

	for _, p := range paragraphs {
		// Убираем HTML-теги
		cleanP := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(p, "")
		words := strings.Fields(cleanP)

		if len(words) > 150 {
			longParagraphs++
		} else if len(words) < 15 && len(words) > 0 {
			shortParagraphs++
		}
	}

	a.SetMetric("paragraph_count", len(paragraphs))
	a.SetMetric("long_paragraphs", longParagraphs)
	a.SetMetric("short_paragraphs", shortParagraphs)

	if longParagraphs > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "long_paragraphs",
			"severity":    "medium",
			"description": "Слишком длинные параграфы затрудняют чтение",
			"count":       longParagraphs,
		})
		a.AddRecommendation("Разбейте длинные параграфы на более короткие (40-70 слов) для улучшения читаемости")
	}

	// Если большинство параграфов короткие, это может указывать на фрагментированный контент
	if len(paragraphs) > 5 && float64(shortParagraphs) > float64(len(paragraphs))*0.7 {
		a.AddIssue(map[string]interface{}{
			"type":        "fragmented_content",
			"severity":    "low",
			"description": "Контент слишком фрагментирован (много коротких параграфов)",
			"short_count": shortParagraphs,
			"total":       len(paragraphs),
		})
		a.AddRecommendation("Объедините некоторые короткие параграфы для более логичной структуры контента")
	}
}

// analyzeDuplicateContent проверяет на дупликаты контента внутри страницы
func (a *ContentAnalyzer) analyzeDuplicateContent(html string) {
	// Находим все значительные текстовые блоки
	textBlockRegex := regexp.MustCompile(`<p[^>]*>(.*?)</p>|<div[^>]*>(.*?)</div>|<section[^>]*>(.*?)</section>`)
	blocks := textBlockRegex.FindAllString(html, -1)

	// Упрощённая проверка на дубликаты
	seenBlocks := make(map[string]int)
	duplicates := 0

	for _, block := range blocks {
		// Убираем HTML-теги и нормализуем пробелы
		cleanBlock := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(block, "")
		cleanBlock = regexp.MustCompile(`\s+`).ReplaceAllString(cleanBlock, " ")
		cleanBlock = strings.TrimSpace(cleanBlock)

		// Проверяем только блоки с значимым содержимым
		if len(cleanBlock) > 50 {
			seenBlocks[cleanBlock]++
			if seenBlocks[cleanBlock] > 1 {
				duplicates++
			}
		}
	}

	a.SetMetric("duplicate_content_blocks", duplicates)

	if duplicates > 0 {
		a.AddIssue(map[string]interface{}{
			"type":        "duplicate_content",
			"severity":    "medium",
			"description": "Обнаружены дублированные блоки контента на странице",
			"count":       duplicates,
		})
		a.AddRecommendation("Удалите или перепишите дублированный контент для улучшения уникальности и пользовательского опыта")
	}
}

// analyzeTextToHtmlRatio анализирует соотношение текста к HTML
func (a *ContentAnalyzer) analyzeTextToHtmlRatio(text string, html string) {
	textLength := len(text)
	htmlLength := len(html)

	ratio := 0.0
	if htmlLength > 0 {
		ratio = float64(textLength) / float64(htmlLength) * 100
	}

	a.SetMetric("text_to_html_ratio", ratio)

	if ratio < 10 {
		a.AddIssue(map[string]interface{}{
			"type":        "low_text_ratio",
			"severity":    "medium",
			"description": "Низкое соотношение текста к HTML",
			"ratio":       ratio,
			"recommended": "более 10%",
		})
		a.AddRecommendation("Увеличьте количество полезного текста относительно HTML-кода для улучшения SEO")
	}
}

// splitIntoSentences разбивает текст на предложения
func splitIntoSentences(text string) []string {
	// Упрощённое разделение на предложения (не идеальное, но достаточное)
	re := regexp.MustCompile(`[.!?]+\s+`)
	sentences := re.Split(text, -1)

	// Фильтруем пустые
	var result []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}

	return result
}

// countSyllables приблизительно подсчитывает количество слогов в тексте
func countSyllables(text string) int {
	syllables := 0
	words := strings.Fields(text)

	for _, word := range words {
		// Упрощенный алгоритм подсчета слогов
		word = strings.ToLower(word)
		word = strings.Trim(word, ".,?!:;()")

		// Для русских слов считаем по гласным
		isRussian := false
		for _, r := range word {
			if unicode.Is(unicode.Cyrillic, r) {
				isRussian = true
				break
			}
		}

		if isRussian {
			russianVowels := []rune{'а', 'е', 'ё', 'и', 'о', 'у', 'ы', 'э', 'ю', 'я'}
			for _, c := range word {
				for _, v := range russianVowels {
					if c == v {
						syllables++
						break
					}
				}
			}
		} else {
			// Для английских слов
			vowels := "aeiouy"
			lastWasVowel := false
			wordSyllables := 0

			// Если слово заканчивается на "e", не считаем это слогом
			if len(word) > 2 && word[len(word)-1] == 'e' {
				word = word[:len(word)-1]
			}

			for _, c := range word {
				isVowel := strings.ContainsRune(vowels, c)
				if isVowel && !lastWasVowel {
					wordSyllables++
				}
				lastWasVowel = isVowel
			}

			if wordSyllables == 0 {
				wordSyllables = 1
			}

			syllables += wordSyllables
		}
	}

	return syllables
}
