package analyzer

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
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
	LighthouseType    AnalyzerType = "lighthouse"
)

// AnalyzerFactory создает анализаторы заданного типа
type AnalyzerFactory struct{}

// CreateAnalyzer создает анализатор заданного типа
func (f *AnalyzerFactory) CreateAnalyzer(analyzerType AnalyzerType, config *config.Config) Analyzer {
	var analyzer Analyzer

	switch analyzerType {
	case SEOType:
		analyzer = NewSEOAnalyzer()
		analyzer.SetPriority(70)
	case PerformanceType:
		analyzer = NewPerformanceAnalyzer()
		analyzer.SetPriority(60)
	case StructureType:
		analyzer = NewStructureAnalyzer()
		analyzer.SetPriority(30)
	case AccessibilityType:
		analyzer = NewAccessibilityAnalyzer()
		analyzer.SetPriority(50)
	case SecurityType:
		analyzer = NewSecurityAnalyzer()
		analyzer.SetPriority(40)
	case MobileType:
		analyzer = NewMobileAnalyzer()
		analyzer.SetPriority(20)
	case ContentType:
		analyzer = NewContentAnalyzer()
		analyzer.SetPriority(10)
	case LighthouseType:
		analyzer = NewLighthouseAnalyzer(config)
		analyzer.SetPriority(100) // Самый высокий приоритет
	default:
		return nil
	}

	return analyzer
}

// AnalyzerManager управляет процессом анализа
type AnalyzerManager struct {
	analyzers map[AnalyzerType]Analyzer
	mu        sync.RWMutex // для потокобезопасного доступа
}

// NewAnalyzerManager создает новый менеджер анализа
func NewAnalyzerManager() *AnalyzerManager {
	return &AnalyzerManager{
		analyzers: make(map[AnalyzerType]Analyzer),
	}
}

// RegisterAnalyzer регистрирует анализатор определенного типа
func (m *AnalyzerManager) RegisterAnalyzer(analyzerType AnalyzerType, analyzer Analyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.analyzers[analyzerType] = analyzer
}

// RegisterAllAnalyzers регистрирует все типы анализаторов
func (m *AnalyzerManager) RegisterAllAnalyzers() {
	config := config.NewConfig()
	factory := &AnalyzerFactory{}

	// Сначала регистрируем Lighthouse, так как он имеет самый высокий приоритет
	if config.LighthouseAPIKey != "" {
		m.RegisterAnalyzer(LighthouseType, factory.CreateAnalyzer(LighthouseType, config))
	}

	m.RegisterAnalyzer(SEOType, factory.CreateAnalyzer(SEOType, config))
	m.RegisterAnalyzer(PerformanceType, factory.CreateAnalyzer(PerformanceType, config))
	m.RegisterAnalyzer(StructureType, factory.CreateAnalyzer(StructureType, config))
	m.RegisterAnalyzer(AccessibilityType, factory.CreateAnalyzer(AccessibilityType, config))
	m.RegisterAnalyzer(SecurityType, factory.CreateAnalyzer(SecurityType, config))
	m.RegisterAnalyzer(MobileType, factory.CreateAnalyzer(MobileType, config))
	m.RegisterAnalyzer(ContentType, factory.CreateAnalyzer(ContentType, config))
}

// RunAnalyzer запускает конкретный анализатор
func (m *AnalyzerManager) RunAnalyzer(ctx context.Context, analyzerType AnalyzerType, data *parser.WebsiteData, prevResults map[AnalyzerType]map[string]interface{}) (map[string]interface{}, error) {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("анализатор типа %s не зарегистрирован", analyzerType)
	}

	return analyzer.Analyze(ctx, data, prevResults)
}

// RunAllAnalyzers запускает все зарегистрированные анализаторы параллельно
func (m *AnalyzerManager) RunAllAnalyzers(ctx context.Context, data *parser.WebsiteData) (map[AnalyzerType]map[string]interface{}, error) {
	m.mu.RLock()
	sortedAnalyzers := m.getSortedAnalyzers()
	m.mu.RUnlock()

	results := make(map[AnalyzerType]map[string]interface{})
	resultsMutex := &sync.RWMutex{}

	// Используем WaitGroup для синхронизации горутин
	var wg sync.WaitGroup

	// Канал для сбора ошибок
	errCh := make(chan error, len(sortedAnalyzers))

	// Запускаем анализаторы в соответствии с приоритетом
	for _, analyzerType := range sortedAnalyzers {
		m.mu.RLock()
		analyzer := m.analyzers[analyzerType]
		m.mu.RUnlock()

		wg.Add(1)

		go func(at AnalyzerType, a Analyzer) {
			defer wg.Done()

			// Собираем текущие результаты для передачи анализатору
			resultsMutex.RLock()
			prevResults := make(map[AnalyzerType]map[string]interface{})
			for k, v := range results {
				prevResults[k] = v
			}
			resultsMutex.RUnlock()

			// Запускаем анализ
			result, err := a.Analyze(ctx, data, prevResults)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("ошибка в анализаторе %s: %w", at, err):
				default:
				}
				return
			}

			// Сохраняем результаты
			resultsMutex.Lock()
			results[at] = result
			resultsMutex.Unlock()
		}(analyzerType, analyzer)
	}

	// Ждем завершения всех анализаторов
	wg.Wait()

	// Проверяем наличие ошибок
	select {
	case err := <-errCh:
		return results, err
	default:
		return results, nil
	}
}

// getSortedAnalyzers возвращает типы анализаторов, отсортированные по приоритету
func (m *AnalyzerManager) getSortedAnalyzers() []AnalyzerType {
	types := make([]AnalyzerType, 0, len(m.analyzers))

	for t := range m.analyzers {
		types = append(types, t)
	}

	sort.Slice(types, func(i, j int) bool {
		return m.analyzers[types[i]].GetPriority() > m.analyzers[types[j]].GetPriority()
	})

	return types
}

// GetAnalyzerIssues возвращает проблемы, найденные анализатором
func (m *AnalyzerManager) GetAnalyzerIssues(analyzerType AnalyzerType) []map[string]interface{} {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return analyzer.GetIssues()
}

// GetAnalyzerRecommendations возвращает рекомендации анализатора
func (m *AnalyzerManager) GetAnalyzerRecommendations(analyzerType AnalyzerType) []string {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return analyzer.GetRecommendations()
}

// GetAllIssues возвращает все проблемы, найденные всеми анализаторами
func (m *AnalyzerManager) GetAllIssues() map[AnalyzerType][]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	issues := make(map[AnalyzerType][]map[string]interface{})

	for analyzerType, analyzer := range m.analyzers {
		issues[analyzerType] = analyzer.GetIssues()
	}

	return issues
}

// GetAllRecommendations возвращает все рекомендации от всех анализаторов
func (m *AnalyzerManager) GetAllRecommendations() map[AnalyzerType][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recommendations := make(map[AnalyzerType][]string)

	for analyzerType, analyzer := range m.analyzers {
		recommendations[analyzerType] = analyzer.GetRecommendations()
	}

	return recommendations
}

// GetOverallScore возвращает общий балл по всем анализаторам
func (m *AnalyzerManager) GetOverallScore() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

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
