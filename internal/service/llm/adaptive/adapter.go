package adaptive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm/feedback"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm/prompts"
)

// PromptAdapter dynamically improves prompts based on feedback
type PromptAdapter struct {
	generator       *prompts.Generator
	feedbackStore   *feedback.FeedbackStore
	promptCache     map[string]string
	mu              sync.RWMutex
	lastRefresh     time.Time
	refreshInterval time.Duration
}

// NewPromptAdapter creates a new prompt adapter
func NewPromptAdapter(generator *prompts.Generator, feedbackStore *feedback.FeedbackStore) *PromptAdapter {
	return &PromptAdapter{
		generator:       generator,
		feedbackStore:   feedbackStore,
		promptCache:     make(map[string]string),
		refreshInterval: 24 * time.Hour,
	}
}

// GeneratePrompt creates an optimized prompt based on request and feedback history
func (a *PromptAdapter) GeneratePrompt(ctx context.Context, request *llm.ContentRequest, promptType prompts.PromptType) (string, string) {
	// Generate prompt ID from request key properties
	promptID := a.generatePromptID(request, string(promptType))

	// Check cache first
	a.mu.RLock()
	cachedPrompt, exists := a.promptCache[promptID]
	a.mu.RUnlock()

	if exists {
		return cachedPrompt, promptID
	}

	// Generate base prompt
	var prompt string

	switch promptType {
	case prompts.PromptTypeHeading:
		prompt = a.generator.HeadingPrompt(request)
	case prompts.PromptTypeMeta:
		prompt = a.generator.MetaDescriptionPrompt(request)
	case prompts.PromptTypeCTA:
		prompt = a.generator.CTAPrompt(request)
	case prompts.PromptTypeContent:
		prompt = a.generator.ContentBlockPrompt(request)
	case prompts.PromptTypeHTML:
		prompt = a.generator.GenerateHTMLPrompt(request.Content, &llm.ContentResponse{
			Title:   request.Title,
			CTAText: request.CTAText,
			Content: request.Content,
		})
	default: // Full content as fallback
		prompt = a.generator.GenerateContentPrompt(request)
	}

	// Apply performance enhancements based on feedback
	enhancedPrompt := a.enhancePromptWithFeedback(ctx, prompt, promptID, string(promptType))

	// Cache the enhanced prompt
	a.mu.Lock()
	a.promptCache[promptID] = enhancedPrompt
	a.mu.Unlock()

	return enhancedPrompt, promptID
}

// generatePromptID generates a unique ID for a prompt
func (a *PromptAdapter) generatePromptID(request *llm.ContentRequest, promptType string) string {
	// Create a hash of the key request properties and prompt type
	hasher := sha256.New()
	hasher.Write([]byte(request.URL))
	hasher.Write([]byte(promptType))
	hasher.Write([]byte(request.Language))
	if request.TargetAudience != "" {
		hasher.Write([]byte(request.TargetAudience))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// enhancePromptWithFeedback improves a prompt based on historical feedback
func (a *PromptAdapter) enhancePromptWithFeedback(ctx context.Context, basePrompt, promptID, promptType string) string {
	// Get feedback for this prompt type
	feedbacks, err := a.feedbackStore.GetFeedbackForPrompt(ctx, promptID)
	if err != nil || len(feedbacks) == 0 {
		return basePrompt // Return original if no feedback or error
	}

	// TODO: Implement more sophisticated enhancement logic based on feedback patterns
	// This would analyze feedback and modify prompt instructions, examples, etc.

	// Currently just returns the original prompt
	return basePrompt
}

// RefreshPromptCache periodically refreshes the prompt cache based on new feedback
func (a *PromptAdapter) RefreshPromptCache(ctx context.Context) {
	// Check if refresh is needed
	if time.Since(a.lastRefresh) < a.refreshInterval {
		return
	}

	// Clear cache and update refresh time
	a.mu.Lock()
	a.promptCache = make(map[string]string)
	a.lastRefresh = time.Now()
	a.mu.Unlock()
}
