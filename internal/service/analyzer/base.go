package analyzer

import (
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// Analyzer - интерфейс для всех анализаторов
type Analyzer interface {
	// Analyze выполняет анализ и возвращает метрики и ошибку
	Analyze(data *parser.WebsiteData) (map[string]interface{}, error)

	// GetMetrics возвращает метрики анализа
	GetMetrics() map[string]interface{}

	// GetIssues возвращает проблемы, найденные при анализе
	GetIssues() []map[string]interface{}

	// GetRecommendations возвращает рекомендации на основе анализа
	GetRecommendations() []string
}

// BaseAnalyzer предоставляет общий функционал для анализаторов
type BaseAnalyzer struct {
	metrics         map[string]interface{}
	issues          []map[string]interface{}
	recommendations []string
}

// NewBaseAnalyzer создает новый базовый анализатор
func NewBaseAnalyzer() *BaseAnalyzer {
	return &BaseAnalyzer{
		metrics:         make(map[string]interface{}),
		issues:          make([]map[string]interface{}, 0),
		recommendations: make([]string, 0),
	}
}

// GetMetrics возвращает метрики анализа
func (a *BaseAnalyzer) GetMetrics() map[string]interface{} {
	return a.metrics
}

// GetIssues возвращает проблемы, найденные при анализе
func (a *BaseAnalyzer) GetIssues() []map[string]interface{} {
	return a.issues
}

// GetRecommendations возвращает рекомендации на основе анализа
func (a *BaseAnalyzer) GetRecommendations() []string {
	return a.recommendations
}

// AddIssue добавляет проблему в анализатор
func (a *BaseAnalyzer) AddIssue(issue map[string]interface{}) {
	a.issues = append(a.issues, issue)
}

// AddRecommendation добавляет рекомендацию в анализатор
func (a *BaseAnalyzer) AddRecommendation(recommendation string) {
	// Проверим, что такой рекомендации еще нет
	for _, rec := range a.recommendations {
		if rec == recommendation {
			return
		}
	}
	a.recommendations = append(a.recommendations, recommendation)
}

// SetMetric устанавливает значение метрики
func (a *BaseAnalyzer) SetMetric(key string, value interface{}) {
	a.metrics[key] = value
}

// CalculateScore вычисляет оценку на основе проблем
func (a *BaseAnalyzer) CalculateScore() float64 {
	score := 100.0
	for _, issue := range a.issues {
		severity := issue["severity"].(string)
		switch severity {
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}
	return score
}
