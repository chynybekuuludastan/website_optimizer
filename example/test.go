package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/analyzer"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

func main() {
	// Получаем URL из аргументов командной строки
	if len(os.Args) < 2 {
		log.Fatal("Использование: lighthouse_usage <URL>")
	}
	url := os.Args[1]

	// Создаем конфигурацию
	cfg := &config.Config{
		LighthouseURL:    "https://www.googleapis.com/pagespeedonline/v5/runPagespeed",
		LighthouseAPIKey: "api_key",
		AnalysisTimeout:  60,
	}

	// Проверяем наличие API ключа
	if cfg.LighthouseAPIKey == "" {
		log.Println("Предупреждение: LIGHTHOUSE_API_KEY не установлен в переменных окружения. Используем Google PageSpeed API без ключа (с ограничениями).")
	}

	// 1. Прямое использование Lighthouse API
	fmt.Println("=== Прямой вызов Lighthouse API ===")
	directLighthouseTest(url, cfg)

	// 2. Использование LighthouseAnalyzer
	fmt.Println("\n=== Использование LighthouseAnalyzer ===")
	analyzerLighthouseTest(url, cfg)
}

// Прямое использование Lighthouse API
func directLighthouseTest(url string, cfg *config.Config) {
	// Создаем клиент
	client := lighthouse.NewClient(cfg.LighthouseURL, cfg.LighthouseAPIKey)

	// Настраиваем опции
	options := lighthouse.DefaultAuditOptions()
	options.FormFactor = lighthouse.FormFactorMobile
	options.Categories = []lighthouse.Category{
		lighthouse.CategoryPerformance,
		lighthouse.CategorySEO,
	}

	// Устанавливаем таймаут
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Запускаем анализ
	fmt.Printf("Анализ URL: %s\n", url)
	startTime := time.Now()

	result, err := client.AnalyzeURL(ctx, url, options)
	if err != nil {
		log.Fatalf("Ошибка анализа: %v", err)
	}

	fmt.Printf("Анализ завершен за %.2f секунд\n", time.Since(startTime).Seconds())
	fmt.Printf("LighthouseVersion: %s\n", result.LighthouseVersion)

	// Выводим оценки по категориям
	fmt.Println("Оценки по категориям:")
	for category, score := range result.Scores {
		fmt.Printf("  - %s: %.1f%%\n", category, score)
	}

	// Выводим основные метрики производительности
	fmt.Println("Ключевые метрики производительности:")
	fmt.Printf("  - First Contentful Paint: %.1f ms\n", result.Metrics.FirstContentfulPaint)
	fmt.Printf("  - Largest Contentful Paint: %.1f ms\n", result.Metrics.LargestContentfulPaint)
	fmt.Printf("  - Total Blocking Time: %.1f ms\n", result.Metrics.TotalBlockingTime)
	fmt.Printf("  - Cumulative Layout Shift: %.3f\n", result.Metrics.CumulativeLayoutShift)
	fmt.Printf("  - Time to Interactive: %.1f ms\n", result.Metrics.TimeToInteractive)

	// Выводим количество проблем и рекомендаций
	fmt.Printf("Найдено проблем: %d\n", len(result.Issues))
	fmt.Printf("Рекомендации: %d\n", len(result.Recommendations))

	// Выводим топ-3 рекомендации
	if len(result.Recommendations) > 0 {
		fmt.Println("Топ рекомендации:")
		maxRecs := 3
		if len(result.Recommendations) < maxRecs {
			maxRecs = len(result.Recommendations)
		}
		for i := 0; i < maxRecs; i++ {
			fmt.Printf("  %d. %s\n", i+1, result.Recommendations[i])
		}
	}
}

// Использование LighthouseAnalyzer
func analyzerLighthouseTest(url string, cfg *config.Config) {
	// Парсим сайт для получения базовой информации
	startTime := time.Now()
	fmt.Printf("Парсинг сайта %s...\n", url)

	websiteData, err := parser.ParseWebsite(url, parser.ParseOptions{
		Timeout: time.Duration(cfg.AnalysisTimeout) * time.Second,
	})
	if err != nil {
		log.Fatalf("Ошибка парсинга: %v", err)
	}

	// Создаем анализатор Lighthouse
	lighthouseAnalyzer := analyzer.NewLighthouseAnalyzer(cfg)

	// Запускаем анализ
	fmt.Println("Запуск Lighthouse анализа...")
	metrics, err := lighthouseAnalyzer.Analyze(websiteData)
	if err != nil {
		log.Fatalf("Ошибка анализа Lighthouse: %v", err)
	}

	fmt.Printf("Анализ завершен за %.2f секунд\n", time.Since(startTime).Seconds())

	// Получаем общую оценку
	score, ok := metrics["score"].(float64)
	if ok {
		fmt.Printf("Общая оценка: %.1f%%\n", score)
	}

	// Выводим оценки по категориям
	if categoryScores, ok := metrics["category_scores"].(map[string]float64); ok {
		fmt.Println("Оценки по категориям:")
		for category, score := range categoryScores {
			fmt.Printf("  - %s: %.1f%%\n", category, score)
		}
	}

	// Выводим проблемы
	issues := lighthouseAnalyzer.GetIssues()
	fmt.Printf("Найдено проблем: %d\n", len(issues))

	// Выводим рекомендации
	recommendations := lighthouseAnalyzer.GetRecommendations()
	fmt.Printf("Рекомендации: %d\n", len(recommendations))

	// Выводим топ-3 рекомендации
	if len(recommendations) > 0 {
		fmt.Println("Топ рекомендации:")
		maxRecs := 3
		if len(recommendations) < maxRecs {
			maxRecs = len(recommendations)
		}
		for i := 0; i < maxRecs; i++ {
			fmt.Printf("  %d. %s\n", i+1, recommendations[i])
		}
	}

	// Экспорт результатов в JSON
	resultsJSON, _ := json.MarshalIndent(metrics, "", "  ")
	fmt.Println("Структура результатов анализа (первые 500 байт):")
	if len(resultsJSON) > 500 {
		fmt.Println(string(resultsJSON[:500]) + "... (сокращено)")
	} else {
		fmt.Println(string(resultsJSON))
	}
}
