package feedback

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

// FeedbackType represents different types of feedback
type FeedbackType string

const (
	FeedbackTypeSuccess     FeedbackType = "success"     // Content was used without modifications
	FeedbackTypeModified    FeedbackType = "modified"    // Content was manually modified before use
	FeedbackTypeRejected    FeedbackType = "rejected"    // Content was completely rejected
	FeedbackTypePerformance FeedbackType = "performance" // Content performance metrics
)

// PromptFeedback stores feedback for a specific prompt
type PromptFeedback struct {
	PromptID    string                 `json:"prompt_id"`
	PromptType  string                 `json:"prompt_type"`
	ContentType string                 `json:"content_type"`
	Feedback    FeedbackType           `json:"feedback"`
	Rating      int                    `json:"rating"` // 1-5 rating
	Comments    string                 `json:"comments,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// FeedbackStore manages storage and retrieval of prompt feedback
type FeedbackStore struct {
	redisClient *redis.Client
	keyPrefix   string
}

// NewFeedbackStore creates a new feedback store
func NewFeedbackStore(client *redis.Client) *FeedbackStore {
	return &FeedbackStore{
		redisClient: client,
		keyPrefix:   "prompt_feedback:",
	}
}

// StoreFeedback saves feedback for a prompt
func (s *FeedbackStore) StoreFeedback(ctx context.Context, feedback *PromptFeedback) error {
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = time.Now()
	}

	// Create key for this feedback entry
	key := s.keyPrefix + feedback.PromptID + ":" + time.Now().Format(time.RFC3339)

	// Convert to JSON
	data, err := json.Marshal(feedback)
	if err != nil {
		return err
	}

	// Store in Redis with 30-day expiration
	return s.redisClient.Set(ctx, key, data, 30*24*time.Hour).Err()
}

// GetFeedbackForPrompt retrieves all feedback for a specific prompt ID
func (s *FeedbackStore) GetFeedbackForPrompt(ctx context.Context, promptID string) ([]*PromptFeedback, error) {
	// Pattern to match all feedback entries for this prompt
	pattern := s.keyPrefix + promptID + ":*"

	// Get keys matching pattern
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var feedbacks []*PromptFeedback

	// Retrieve each feedback entry
	for _, key := range keys {
		data, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var feedback PromptFeedback
		if err := json.Unmarshal([]byte(data), &feedback); err != nil {
			continue
		}

		feedbacks = append(feedbacks, &feedback)
	}

	return feedbacks, nil
}

// GetFeedbackStats returns statistics for a prompt type
func (s *FeedbackStore) GetFeedbackStats(ctx context.Context, promptType string) (map[string]interface{}, error) {
	// Pattern to match all feedback entries for this prompt type
	pattern := s.keyPrefix + "*"

	// Get keys matching pattern
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total":          0,
		"ratings":        map[int]int{},
		"feedback_types": map[string]int{},
	}

	// Process each feedback entry
	for _, key := range keys {
		data, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var feedback PromptFeedback
		if err := json.Unmarshal([]byte(data), &feedback); err != nil {
			continue
		}

		// Only process feedback for matching prompt type
		if feedback.PromptType == promptType {
			stats["total"] = stats["total"].(int) + 1

			// Update rating counts
			ratings := stats["ratings"].(map[int]int)
			ratings[feedback.Rating]++

			// Update feedback type counts
			feedbackTypes := stats["feedback_types"].(map[string]int)
			feedbackTypes[string(feedback.Feedback)]++
		}
	}

	return stats, nil
}
