package tokens

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-redis/redis/v8"
)

// Models maps model names to pricing information
var Models = map[string]ModelInfo{
	"gpt-4": {
		TokensPerPromptDollar: 1000.0 / 0.03, // $0.03 per 1K prompt tokens
		TokensPerOutputDollar: 1000.0 / 0.06, // $0.06 per 1K completion tokens
		MaxContextTokens:      8192,
		Name:                  "gpt-4",
		Provider:              "openai",
	},
	"gpt-4-turbo": {
		TokensPerPromptDollar: 1000.0 / 0.01, // $0.01 per 1K prompt tokens
		TokensPerOutputDollar: 1000.0 / 0.03, // $0.03 per 1K completion tokens
		MaxContextTokens:      128000,
		Name:                  "gpt-4-turbo",
		Provider:              "openai",
	},
	"gpt-3.5-turbo": {
		TokensPerPromptDollar: 1000.0 / 0.0015, // $0.0015 per 1K prompt tokens
		TokensPerOutputDollar: 1000.0 / 0.002,  // $0.002 per 1K completion tokens
		MaxContextTokens:      16385,
		Name:                  "gpt-3.5-turbo",
		Provider:              "openai",
	},
	"gemini-1.5-flash": {
		TokensPerPromptDollar: 1000.0 / 0.00035, // $0.00035 per 1K input tokens
		TokensPerOutputDollar: 1000.0 / 0.00035, // $0.00035 per 1K output tokens
		MaxContextTokens:      1000000,
		Name:                  "gemini-1.5-flash",
		Provider:              "gemini",
	},
	"gemini-1.5-pro": {
		TokensPerPromptDollar: 1000.0 / 0.00175, // $0.00175 per 1K input tokens
		TokensPerOutputDollar: 1000.0 / 0.00175, // $0.00175 per 1K output tokens
		MaxContextTokens:      1000000,
		Name:                  "gemini-1.5-pro",
		Provider:              "gemini",
	},
}

// ModelInfo contains pricing information for a model
type ModelInfo struct {
	TokensPerPromptDollar float64 // Tokens per dollar for input
	TokensPerOutputDollar float64 // Tokens per dollar for output
	MaxContextTokens      int     // Maximum context length
	Name                  string  // Model name
	Provider              string  // Provider name
}

// UsageEntry represents a token usage entry
type UsageEntry struct {
	Timestamp        time.Time
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	PromptCost       float64
	CompletionCost   float64
	TotalCost        float64
	RequestID        string
	ContentType      string
}

// BudgetTracker tracks token usage and costs
type BudgetTracker struct {
	redisClient *redis.Client
	keyPrefix   string
	dailyBudget float64
	currentDay  string
	dailyUsage  float64
	mu          sync.RWMutex
}

// NewBudgetTracker creates a new budget tracker
func NewBudgetTracker(client *redis.Client, dailyBudget float64) *BudgetTracker {
	tracker := &BudgetTracker{
		redisClient: client,
		keyPrefix:   "llm_tokens:",
		dailyBudget: dailyBudget,
		currentDay:  time.Now().Format("2006-01-02"),
	}

	// Initialize daily usage from Redis
	tracker.refreshDailyUsage()

	return tracker
}

// TokensToCost converts tokens to cost for a given model
func (t *BudgetTracker) TokensToCost(model string, promptTokens, completionTokens int) (float64, float64, float64) {
	modelInfo, ok := Models[model]
	if !ok {
		// Use GPT-4 pricing as fallback
		modelInfo = Models["gpt-4"]
	}

	promptCost := float64(promptTokens) / modelInfo.TokensPerPromptDollar
	completionCost := float64(completionTokens) / modelInfo.TokensPerOutputDollar
	totalCost := promptCost + completionCost

	return promptCost, completionCost, totalCost
}

// RecordUsage records token usage
func (t *BudgetTracker) RecordUsage(entry UsageEntry) error {
	// Calculate costs if not already provided
	if entry.PromptCost == 0 || entry.CompletionCost == 0 {
		entry.PromptCost, entry.CompletionCost, entry.TotalCost =
			t.TokensToCost(entry.Model, entry.PromptTokens, entry.CompletionTokens)
	}

	// Format key with date for daily tracking
	day := entry.Timestamp.Format("2006-01-02")
	// key := t.keyPrefix + day

	// Update daily usage
	t.mu.Lock()
	if day == t.currentDay {
		t.dailyUsage += entry.TotalCost
	} else {
		t.currentDay = day
		t.refreshDailyUsage()
	}
	t.mu.Unlock()

	// Store in Redis (implementation depends on how you want to store/retrieve usage data)
	// This is a simplified example - in a real implementation, you might use a Redis list,
	// sorted set, or hash to efficiently store and query usage data

	return nil
}

// IsBudgetExceeded checks if daily budget is exceeded
func (t *BudgetTracker) IsBudgetExceeded() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.dailyUsage >= t.dailyBudget
}

// GetRemainingBudget returns the remaining daily budget
func (t *BudgetTracker) GetRemainingBudget() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.dailyUsage >= t.dailyBudget {
		return 0
	}
	return t.dailyBudget - t.dailyUsage
}

// refreshDailyUsage refreshes the daily usage from Redis
func (t *BudgetTracker) refreshDailyUsage() {
	// In a real implementation, this would query Redis to sum up
	// all usage for the current day
	t.dailyUsage = 0
}

// EstimateTokens estimates the number of tokens in a string
// This is a very rough approximation; different models tokenize differently
func EstimateTokens(text string) int {
	// Simplified token estimation:
	// Roughly 4 characters per token for English text
	return utf8.RuneCountInString(text) / 4
}

// CalculateContextSize calculates estimated token size for request/response
func CalculateContextSize(prompt, completion string) (int, int) {
	promptTokens := EstimateTokens(prompt)
	completionTokens := EstimateTokens(completion)

	return promptTokens, completionTokens
}

// TruncatePromptToFit ensures a prompt fits within a model's context window
func TruncatePromptToFit(prompt string, modelName string, reserveForCompletion int) string {
	modelInfo, ok := Models[modelName]
	if !ok {
		// Default to GPT-3.5 limits if model not found
		modelInfo = Models["gpt-3.5-turbo"]
	}

	// Calculate available tokens
	availableTokens := modelInfo.MaxContextTokens - reserveForCompletion

	// Estimate current prompt tokens
	promptTokens := EstimateTokens(prompt)

	// If it fits, return as is
	if promptTokens <= availableTokens {
		return prompt
	}

	// Otherwise truncate
	// This is a simple character-based truncation; a more sophisticated
	// implementation would be aware of token boundaries and preserve
	// important parts of the prompt

	// Calculate approximate character limit (4 chars per token is a rough estimate)
	charLimit := availableTokens * 4

	// Simple truncation
	if len(prompt) > charLimit {
		// Try to truncate at sentence or paragraph boundary
		parts := strings.Split(prompt, "\n\n")
		result := ""

		for _, part := range parts {
			if len(result)+len(part)+2 <= charLimit {
				if result != "" {
					result += "\n\n"
				}
				result += part
			} else {
				break
			}
		}

		return result
	}

	return prompt
}
