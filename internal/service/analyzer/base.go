package analyzer

import (
	"context"
	"sync"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/lighthouse"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// Analyzer - интерфейс для всех анализаторов
type Analyzer interface {
	// Analyze выполняет анализ и возвращает метрики и ошибку
	// prevResults содержит результаты других анализаторов
	Analyze(ctx context.Context, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error)

	// GetMetrics возвращает метрики анализа
	GetMetrics() map[string]interface{}

	// GetIssues возвращает проблемы, найденные при анализе
	GetIssues() []map[string]interface{}

	// GetRecommendations возвращает рекомендации на основе анализа
	GetRecommendations() []string

	// GetType возвращает тип анализатора
	GetType() AnalyzerType

	// SetPriority устанавливает приоритет выполнения анализатора
	SetPriority(priority int)

	// GetPriority возвращает приоритет анализатора
	GetPriority() int
}

// BaseAnalyzer предоставляет общий функционал для анализаторов
type BaseAnalyzer struct {
	metrics         map[string]interface{}
	issues          []map[string]interface{}
	recommendations []string
	priority        int
	analyzerType    AnalyzerType
	mu              sync.RWMutex // для потокобезопасного доступа к данным
}

// NewBaseAnalyzer создает новый базовый анализатор
func NewBaseAnalyzer(analyzerType AnalyzerType) *BaseAnalyzer {
	return &BaseAnalyzer{
		metrics:         make(map[string]interface{}),
		issues:          make([]map[string]interface{}, 0),
		recommendations: make([]string, 0),
		priority:        0,
		analyzerType:    analyzerType,
	}
}

// GetMetrics возвращает метрики анализа
func (a *BaseAnalyzer) GetMetrics() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]interface{}, len(a.metrics))
	for k, v := range a.metrics {
		result[k] = v
	}
	return result
}

// GetIssues возвращает проблемы, найденные при анализе
func (a *BaseAnalyzer) GetIssues() []map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.issues
}

// GetRecommendations возвращает рекомендации на основе анализа
func (a *BaseAnalyzer) GetRecommendations() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.recommendations
}

// AddIssue добавляет проблему в анализатор
func (a *BaseAnalyzer) AddIssue(issue map[string]interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.issues = append(a.issues, issue)
}

// AddRecommendation добавляет рекомендацию в анализатор
func (a *BaseAnalyzer) AddRecommendation(recommendation string) {
	a.mu.Lock()
	defer a.mu.Unlock()

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
	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics[key] = value
}

// CalculateScore вычисляет оценку на основе проблем
func (a *BaseAnalyzer) CalculateScore() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

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

// SetPriority устанавливает приоритет анализатора
func (a *BaseAnalyzer) SetPriority(priority int) {
	a.priority = priority
}

// GetPriority возвращает приоритет анализатора
func (a *BaseAnalyzer) GetPriority() int {
	return a.priority
}

// GetType возвращает тип анализатора
func (a *BaseAnalyzer) GetType() AnalyzerType {
	return a.analyzerType
}

// extractAuditsData извлекает данные аудитов из результатов Lighthouse
func extractAuditsData(lighthouseResults map[string]interface{}, auditTypes map[string]bool) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	if audits, ok := lighthouseResults["audits"]; ok {
		if auditMap, ok := audits.(map[string]interface{}); ok {
			for id, auditData := range auditMap {
				// Проверяем, относится ли этот аудит к интересующим нас типам
				if !auditTypes[id] {
					continue
				}

				if auditObj, ok := auditData.(map[string]interface{}); ok {
					result[id] = auditObj
				}
			}
		}
	}

	return result
}

// extractAuditsData извлекает данные аудитов по заданному типу
func extractLighthouseAuditsData(audits map[string]lighthouse.Audit, auditTypes map[string]bool) map[string]lighthouse.Audit {
	result := make(map[string]lighthouse.Audit)

	for id, audit := range audits {
		// Проверяем, относится ли этот аудит к интересующим нас типам
		if !auditTypes[id] {
			continue
		}

		result[id] = audit
	}

	return result
}

// getAuditIssues получает проблемы из аудитов Lighthouse
func getAuditIssues(auditsData map[string]map[string]interface{}, minScoreThreshold float64) []map[string]interface{} {
	var issues []map[string]interface{}

	for id, audit := range auditsData {
		var score float64
		if scoreVal, ok := audit["score"].(float64); ok {
			score = scoreVal
		}

		// Если оценка ниже порога и не равна -1 (информационные аудиты)
		if score < minScoreThreshold && score >= 0 {
			title, _ := audit["title"].(string)
			description, _ := audit["description"].(string)

			issues = append(issues, map[string]interface{}{
				"type":        "lighthouse_" + id,
				"severity":    getSeverityFromScore(score),
				"description": title,
				"details":     description,
				"score":       score,
			})
		}
	}

	return issues
}
