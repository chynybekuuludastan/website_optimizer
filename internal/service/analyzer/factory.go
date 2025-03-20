package analyzer

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/config"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// ProgressUpdate represents an update about the progress of an analyzer
type ProgressUpdate struct {
	AnalyzerType   string                 // The type of analyzer reporting progress
	Progress       float64                // Progress percentage (0-100)
	Message        string                 // Description of current state
	Details        map[string]interface{} // Additional details about the progress
	PartialResults map[string]interface{} // Partial analysis results, if available
	Timestamp      time.Time              // When this update was generated
}

// AnalyzerType defines the type of analyzer
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

// All analyzer types in a slice for easy iteration
var AllAnalyzerTypes = []AnalyzerType{
	LighthouseType, // Highest priority first
	SEOType,
	PerformanceType,
	AccessibilityType,
	SecurityType,
	StructureType,
	MobileType,
	ContentType,
}

// AnalyzerFactory creates analyzers of a specified type
type AnalyzerFactory struct {
	config *config.Config
}

// NewAnalyzerFactory creates a new factory with the provided configuration
func NewAnalyzerFactory(cfg *config.Config) *AnalyzerFactory {
	return &AnalyzerFactory{
		config: cfg,
	}
}

// CreateAnalyzer creates an analyzer of the specified type
func (f *AnalyzerFactory) CreateAnalyzer(analyzerType AnalyzerType) (Analyzer, error) {
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
		analyzer = NewLighthouseAnalyzer(f.config)
		analyzer.SetPriority(100) // Highest priority
	default:
		return nil, fmt.Errorf("unknown analyzer type: %s", analyzerType)
	}

	return analyzer, nil
}

// AnalyzerManager manages the analysis process
type AnalyzerManager struct {
	analyzers         map[AnalyzerType]Analyzer
	config            *config.Config
	factory           *AnalyzerFactory
	mu                sync.RWMutex // For thread-safe access
	progressCallback  func(ProgressUpdate)
	dependencyGraph   map[AnalyzerType][]AnalyzerType // Defines which analyzer depends on which
	isExecuting       bool
	executingMu       sync.Mutex
	analysisStartTime time.Time
}

// NewAnalyzerManager creates a new analysis manager
func NewAnalyzerManager() *AnalyzerManager {
	config := config.NewConfig()

	manager := &AnalyzerManager{
		analyzers:       make(map[AnalyzerType]Analyzer),
		config:          config,
		factory:         NewAnalyzerFactory(config),
		dependencyGraph: buildDefaultDependencyGraph(),
		isExecuting:     false,
	}

	return manager
}

// buildDefaultDependencyGraph creates the default dependency graph for analyzers
func buildDefaultDependencyGraph() map[AnalyzerType][]AnalyzerType {
	// Define which analyzers depend on results from other analyzers
	dependencies := make(map[AnalyzerType][]AnalyzerType)

	// Lighthouse provides data for many other analyzers
	dependencies[SEOType] = []AnalyzerType{LighthouseType}
	dependencies[PerformanceType] = []AnalyzerType{LighthouseType}
	dependencies[AccessibilityType] = []AnalyzerType{LighthouseType}
	dependencies[SecurityType] = []AnalyzerType{LighthouseType}
	dependencies[MobileType] = []AnalyzerType{LighthouseType}
	dependencies[ContentType] = []AnalyzerType{LighthouseType}

	return dependencies
}

// RegisterAnalyzer registers an analyzer of a specific type
func (m *AnalyzerManager) RegisterAnalyzer(analyzerType AnalyzerType, analyzer Analyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.analyzers[analyzerType] = analyzer
	log.Printf("Registered analyzer: %s with priority %d", analyzerType, analyzer.GetPriority())
}

// RegisterAllAnalyzers registers all types of analyzers
func (m *AnalyzerManager) RegisterAllAnalyzers() {
	// First check if Lighthouse API key is available
	if m.config.LighthouseAPIKey != "" {
		analyzer, err := m.factory.CreateAnalyzer(LighthouseType)
		if err == nil {
			m.RegisterAnalyzer(LighthouseType, analyzer)
		} else {
			log.Printf("Failed to create Lighthouse analyzer: %v", err)
		}
	} else {
		log.Println("Lighthouse API key not provided, skipping Lighthouse analyzer")
	}

	// Register all other analyzers
	for _, aType := range []AnalyzerType{
		SEOType, PerformanceType, StructureType,
		AccessibilityType, SecurityType, MobileType, ContentType,
	} {
		analyzer, err := m.factory.CreateAnalyzer(aType)
		if err == nil {
			m.RegisterAnalyzer(aType, analyzer)
		} else {
			log.Printf("Failed to create analyzer %s: %v", aType, err)
		}
	}
}

// RunAnalyzer runs a specific analyzer
func (m *AnalyzerManager) RunAnalyzer(
	ctx context.Context,
	analyzerType AnalyzerType,
	data *parser.WebsiteData,
	prevResults map[AnalyzerType]map[string]interface{},
) (map[string]interface{}, error) {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("analyzer of type %s not registered", analyzerType)
	}

	// Emit progress update before starting
	if m.progressCallback != nil {
		m.progressCallback(ProgressUpdate{
			AnalyzerType: string(analyzerType),
			Progress:     0.0,
			Message:      fmt.Sprintf("Starting %s analysis", analyzerType),
			Timestamp:    time.Now(),
		})
	}

	// Run the analysis
	startTime := time.Now()
	result, err := analyzer.Analyze(ctx, data, prevResults)

	// Emit progress update upon completion
	if m.progressCallback != nil {
		progress := 100.0
		message := fmt.Sprintf("%s analysis completed in %v", analyzerType, time.Since(startTime))

		if err != nil {
			message = fmt.Sprintf("%s analysis failed: %v", analyzerType, err)
		}

		m.progressCallback(ProgressUpdate{
			AnalyzerType:   string(analyzerType),
			Progress:       progress,
			Message:        message,
			PartialResults: result, // Include full results at completion
			Timestamp:      time.Now(),
		})
	}

	return result, err
}

// RunAllAnalyzers runs all registered analyzers in parallel according to dependencies
func (m *AnalyzerManager) RunAllAnalyzers(ctx context.Context, data *parser.WebsiteData) (map[AnalyzerType]map[string]interface{}, error) {
	m.executingMu.Lock()
	if m.isExecuting {
		m.executingMu.Unlock()
		return nil, fmt.Errorf("analysis already in progress")
	}
	m.isExecuting = true
	m.analysisStartTime = time.Now()
	m.executingMu.Unlock()

	defer func() {
		m.executingMu.Lock()
		m.isExecuting = false
		m.executingMu.Unlock()
	}()

	m.mu.RLock()
	sortedAnalyzers := m.getSortedAnalyzers()
	m.mu.RUnlock()

	results := make(map[AnalyzerType]map[string]interface{})
	resultsMutex := &sync.RWMutex{}

	// Track analyzers that have completed
	completed := make(map[AnalyzerType]bool)
	completedMutex := &sync.Mutex{}

	// Group analyzers based on execution layers
	executionLayers := m.buildExecutionLayers(sortedAnalyzers)

	// Create a channel to collect errors
	errorChan := make(chan error, len(sortedAnalyzers))

	// Process each layer sequentially
	for layerIndex, layer := range executionLayers {
		log.Printf("Processing execution layer %d with %d analyzers", layerIndex+1, len(layer))

		// Notify progress about layer start
		if m.progressCallback != nil {
			m.progressCallback(ProgressUpdate{
				AnalyzerType: "manager",
				Progress:     float64(layerIndex) / float64(len(executionLayers)) * 100.0,
				Message:      fmt.Sprintf("Processing analysis layer %d of %d", layerIndex+1, len(executionLayers)),
				Timestamp:    time.Now(),
			})
		}

		// Execute all analyzers in this layer in parallel
		var layerWg sync.WaitGroup

		for _, analyzerType := range layer {
			m.mu.RLock()
			analyzer, exists := m.analyzers[analyzerType]
			m.mu.RUnlock()

			if !exists {
				log.Printf("Warning: Analyzer %s not found, skipping", analyzerType)
				continue
			}

			layerWg.Add(1)

			go func(at AnalyzerType, a Analyzer) {
				defer layerWg.Done()

				// Check if context is cancelled
				select {
				case <-ctx.Done():
					errorChan <- fmt.Errorf("analyzer %s cancelled: %w", at, ctx.Err())
					return
				default:
					// Continue with analysis
				}

				// Get the current set of results to pass to this analyzer
				resultsMutex.RLock()
				prevResults := make(map[AnalyzerType]map[string]interface{})
				for k, v := range results {
					prevResults[k] = v
				}
				resultsMutex.RUnlock()

				// Report start of analyzer
				if m.progressCallback != nil {
					m.progressCallback(ProgressUpdate{
						AnalyzerType: string(at),
						Progress:     0.0,
						Message:      fmt.Sprintf("Starting %s analysis", at),
						Timestamp:    time.Now(),
					})
				}

				// Execute the analyzer
				startTime := time.Now()
				result, err := a.Analyze(ctx, data, prevResults)
				duration := time.Since(startTime)

				// Check for errors
				if err != nil {
					errorMsg := fmt.Sprintf("error in analyzer %s: %v", at, err)
					log.Println(errorMsg)
					errorChan <- fmt.Errorf(errorMsg)

					// Report failure
					if m.progressCallback != nil {
						m.progressCallback(ProgressUpdate{
							AnalyzerType: string(at),
							Progress:     100.0,
							Message:      fmt.Sprintf("Failed: %v", err),
							Timestamp:    time.Now(),
						})
					}
					return
				}

				// Save results
				resultsMutex.Lock()
				results[at] = result
				resultsMutex.Unlock()

				// Mark as completed
				completedMutex.Lock()
				completed[at] = true
				completedMutex.Unlock()

				// Report completion
				if m.progressCallback != nil {
					m.progressCallback(ProgressUpdate{
						AnalyzerType:   string(at),
						Progress:       100.0,
						Message:        fmt.Sprintf("Completed in %v", duration),
						PartialResults: result,
						Timestamp:      time.Now(),
						Details: map[string]interface{}{
							"duration_ms": duration.Milliseconds(),
							"layer":       layerIndex + 1,
							"priority":    a.GetPriority(),
						},
					})
				}

				log.Printf("Analyzer %s completed in %v", at, duration)
			}(analyzerType, analyzer)
		}

		// Wait for all analyzers in this layer to complete
		layerWg.Wait()

		// Check for errors after each layer
		select {
		case err := <-errorChan:
			return results, err
		default:
			// No errors, continue to next layer
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			// Continue processing
		}
	}

	// Report overall completion
	if m.progressCallback != nil {
		m.progressCallback(ProgressUpdate{
			AnalyzerType: "manager",
			Progress:     100.0,
			Message:      fmt.Sprintf("All analyzers completed in %v", time.Since(m.analysisStartTime)),
			Timestamp:    time.Now(),
			Details: map[string]interface{}{
				"total_analyzers":   len(sortedAnalyzers),
				"completed":         len(completed),
				"total_duration_ms": time.Since(m.analysisStartTime).Milliseconds(),
				"overall_score":     m.calculateOverallScore(results),
				"execution_layers":  len(executionLayers),
			},
		})
	}

	// Return the results
	return results, nil
}

// calculateOverallScore calculates the overall score from analyzer results
func (m *AnalyzerManager) calculateOverallScore(results map[AnalyzerType]map[string]interface{}) float64 {
	var totalScore float64
	count := 0

	for _, result := range results {
		if score, ok := result["score"].(float64); ok {
			totalScore += score
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalScore / float64(count)
}

// buildExecutionLayers organizes analyzers into execution layers based on dependencies
func (m *AnalyzerManager) buildExecutionLayers(sortedAnalyzers []AnalyzerType) [][]AnalyzerType {
	layers := [][]AnalyzerType{}
	remaining := make(map[AnalyzerType]bool)
	processed := make(map[AnalyzerType]bool)

	// Initialize remaining analyzers
	for _, at := range sortedAnalyzers {
		remaining[at] = true
	}

	// Process layers until all analyzers are placed
	for len(remaining) > 0 {
		currentLayer := []AnalyzerType{}

		// Find analyzers that can be executed in this layer
		for at := range remaining {
			canExecute := true

			// Check if all dependencies are satisfied
			if deps, ok := m.dependencyGraph[at]; ok {
				for _, dep := range deps {
					if !processed[dep] && m.analyzerExists(dep) {
						canExecute = false
						break
					}
				}
			}

			if canExecute {
				currentLayer = append(currentLayer, at)
			}
		}

		// If no analyzers can be executed in this layer, break dependency cycle
		if len(currentLayer) == 0 {
			// Find analyzer with highest priority
			var highestPriorityAnalyzer AnalyzerType
			highestPriority := -1

			for at := range remaining {
				m.mu.RLock()
				analyzer, exists := m.analyzers[at]
				m.mu.RUnlock()

				if exists && analyzer.GetPriority() > highestPriority {
					highestPriority = analyzer.GetPriority()
					highestPriorityAnalyzer = at
				}
			}

			if highestPriority >= 0 {
				currentLayer = append(currentLayer, highestPriorityAnalyzer)
				log.Printf("Warning: Breaking dependency cycle by executing %s", highestPriorityAnalyzer)
			} else {
				// This should never happen if analyzers are properly registered
				log.Println("Error: No analyzer found to break dependency cycle")
				break
			}
		}

		// Sort the current layer by priority
		sort.Slice(currentLayer, func(i, j int) bool {
			m.mu.RLock()
			a1 := m.analyzers[currentLayer[i]]
			a2 := m.analyzers[currentLayer[j]]
			m.mu.RUnlock()
			return a1.GetPriority() > a2.GetPriority()
		})

		// Add layer and mark analyzers as processed
		layers = append(layers, currentLayer)
		for _, at := range currentLayer {
			delete(remaining, at)
			processed[at] = true
		}
	}

	return layers
}

// analyzerExists checks if an analyzer is registered
func (m *AnalyzerManager) analyzerExists(analyzerType AnalyzerType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.analyzers[analyzerType]
	return exists
}

// getSortedAnalyzers returns analyzer types sorted by priority
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

// GetAnalyzerIssues returns issues found by a specific analyzer
func (m *AnalyzerManager) GetAnalyzerIssues(analyzerType AnalyzerType) []map[string]interface{} {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return analyzer.GetIssues()
}

// GetAnalyzerRecommendations returns recommendations from a specific analyzer
func (m *AnalyzerManager) GetAnalyzerRecommendations(analyzerType AnalyzerType) []string {
	m.mu.RLock()
	analyzer, exists := m.analyzers[analyzerType]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return analyzer.GetRecommendations()
}

// GetAllIssues returns all issues found by all analyzers
func (m *AnalyzerManager) GetAllIssues() map[AnalyzerType][]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	issues := make(map[AnalyzerType][]map[string]interface{})

	for analyzerType, analyzer := range m.analyzers {
		issues[analyzerType] = analyzer.GetIssues()
	}

	return issues
}

// GetAllRecommendations returns all recommendations from all analyzers
func (m *AnalyzerManager) GetAllRecommendations() map[AnalyzerType][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recommendations := make(map[AnalyzerType][]string)

	for analyzerType, analyzer := range m.analyzers {
		recommendations[analyzerType] = analyzer.GetRecommendations()
	}

	return recommendations
}

// GetOverallScore returns the overall score across all analyzers
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

// SetProgressCallback sets the callback function for progress updates
func (m *AnalyzerManager) SetProgressCallback(callback func(ProgressUpdate)) {
	m.progressCallback = callback
}

// IsAnalysisInProgress checks if an analysis is currently running
func (m *AnalyzerManager) IsAnalysisInProgress() bool {
	m.executingMu.Lock()
	defer m.executingMu.Unlock()
	return m.isExecuting
}

// GetAnalysisDuration returns the duration of the current or last analysis
func (m *AnalyzerManager) GetAnalysisDuration() time.Duration {
	m.executingMu.Lock()
	defer m.executingMu.Unlock()

	if !m.analysisStartTime.IsZero() {
		return time.Since(m.analysisStartTime)
	}

	return 0
}
