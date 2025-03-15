package analyzer

import (
	"fmt"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// AnalyzerType определяет тип анализатора
type AnalyzerType string

const (
	SEOType           AnalyzerType = "seo"
	PerformanceType   AnalyzerType = "performance"
	StructureType     AnalyzerType = "structure"
	AccessibilityType AnalyzerType = "accessibility"
	SecurityType      AnalyzerType = "security"
	MobileType        AnalyzerType = "mobile"
	ContentType       AnalyzerType = "content"
)

// AnalyzerFactory создает анализаторы заданного типа
type AnalyzerFactory struct{}

// CreateAnalyzer создает анализатор заданного типа
func (f *AnalyzerFactory) CreateAnalyzer(analyzerType AnalyzerType) Analyzer {
	switch analyzerType {
	case SEOType:
		return NewSEOAnalyzer()
	case PerformanceType:
		return NewPerformanceAnalyzer()
	case StructureType:
		return NewStructureAnalyzer()
	case AccessibilityType:
		return NewAccessibilityAnalyzer()
	case SecurityType:
		return NewSecurityAnalyzer()
	case MobileType:
		return NewMobileAnalyzer()
	case ContentType:
		return NewContentAnalyzer()
	default:
		return nil
	}
}

// AnalyzerManager управляет процессом анализа
type AnalyzerManager struct {
	analyzers map[AnalyzerType]Analyzer
}

// NewAnalyzerManager создает новый менеджер анализа
func NewAnalyzerManager() *AnalyzerManager {
	return &AnalyzerManager{
		analyzers: make(map[AnalyzerType]Analyzer),
	}
}

// RegisterAnalyzer регистрирует анализатор определенного типа
func (m *AnalyzerManager) RegisterAnalyzer(analyzerType AnalyzerType, analyzer Analyzer) {
	m.analyzers[analyzerType] = analyzer
}

// RegisterAllAnalyzers регистрирует все типы анализаторов
func (m *AnalyzerManager) RegisterAllAnalyzers() {
	factory := &AnalyzerFactory{}

	m.RegisterAnalyzer(SEOType, factory.CreateAnalyzer(SEOType))
	m.RegisterAnalyzer(PerformanceType, factory.CreateAnalyzer(PerformanceType))
	m.RegisterAnalyzer(StructureType, factory.CreateAnalyzer(StructureType))
	m.RegisterAnalyzer(AccessibilityType, factory.CreateAnalyzer(AccessibilityType))
	m.RegisterAnalyzer(SecurityType, factory.CreateAnalyzer(SecurityType))
	m.RegisterAnalyzer(MobileType, factory.CreateAnalyzer(MobileType))
	m.RegisterAnalyzer(ContentType, factory.CreateAnalyzer(ContentType))
}

// RunAnalyzer запускает конкретный анализатор
func (m *AnalyzerManager) RunAnalyzer(analyzerType AnalyzerType, data *parser.WebsiteData) (map[string]interface{}, error) {
	analyzer, exists := m.analyzers[analyzerType]
	if !exists {
		return nil, fmt.Errorf("анализатор типа %s не зарегистрирован", analyzerType)
	}

	return analyzer.Analyze(data)
}

// RunAllAnalyzers запускает все зарегистрированные анализаторы
func (m *AnalyzerManager) RunAllAnalyzers(data *parser.WebsiteData) (map[AnalyzerType]map[string]interface{}, error) {
	results := make(map[AnalyzerType]map[string]interface{})

	for analyzerType, analyzer := range m.analyzers {
		result, err := analyzer.Analyze(data)
		if err != nil {
			return results, err
		}
		results[analyzerType] = result
	}

	return results, nil
}

// GetAnalyzerIssues возвращает проблемы, найденные анализатором
func (m *AnalyzerManager) GetAnalyzerIssues(analyzerType AnalyzerType) []map[string]interface{} {
	analyzer, exists := m.analyzers[analyzerType]
	if !exists {
		return nil
	}

	return analyzer.GetIssues()
}

// GetAnalyzerRecommendations возвращает рекомендации анализатора
func (m *AnalyzerManager) GetAnalyzerRecommendations(analyzerType AnalyzerType) []string {
	analyzer, exists := m.analyzers[analyzerType]
	if !exists {
		return nil
	}

	return analyzer.GetRecommendations()
}

// GetAllIssues возвращает все проблемы, найденные всеми анализаторами
func (m *AnalyzerManager) GetAllIssues() map[AnalyzerType][]map[string]interface{} {
	issues := make(map[AnalyzerType][]map[string]interface{})

	for analyzerType, analyzer := range m.analyzers {
		issues[analyzerType] = analyzer.GetIssues()
	}

	return issues
}

// GetAllRecommendations возвращает все рекомендации от всех анализаторов
func (m *AnalyzerManager) GetAllRecommendations() map[AnalyzerType][]string {
	recommendations := make(map[AnalyzerType][]string)

	for analyzerType, analyzer := range m.analyzers {
		recommendations[analyzerType] = analyzer.GetRecommendations()
	}

	return recommendations
}

// GetOverallScore возвращает общий балл по всем анализаторам
func (m *AnalyzerManager) GetOverallScore() float64 {
	var totalScore float64
	count := 0

	for _, analyzer := range m.analyzers {
		metrics := analyzer.GetMetrics()
		if score, ok := metrics["score"].(float64); ok {
			totalScore += score
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalScore / float64(count)
}
