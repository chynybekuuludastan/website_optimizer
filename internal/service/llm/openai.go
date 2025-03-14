package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	OpenAIAPIURL = "https://api.openai.com/v1/chat/completions"
)

// OpenAIClient represents an OpenAI API client
type OpenAIClient struct {
	APIKey  string
	Timeout time.Duration
	Model   string
}

// Message represents a single message in a conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents the request to OpenAI's chat completion API
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse represents the response from OpenAI's chat completion API
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
		Model:   "gpt-4",
	}
}

// GenerateContentImprovements generates improved content using OpenAI
func (c *OpenAIClient) GenerateContentImprovements(
	url string,
	title string,
	ctaButton string,
	content string,
) (map[string]string, error) {
	prompt := fmt.Sprintf(`
На основе анализа сайта %s, предложите улучшенные версии заголовков, CTA-кнопок и текстового
контента для повышения конверсии. Учитывайте текущий контент:
- заголовок: "%s"
- CTA: "%s"
- текст: "%s"

Формат ответа: JSON с полями 'heading', 'cta_button', 'improved_content'.
`, url, title, ctaButton, content)

	messages := []Message{
		{Role: "system", Content: "Вы - эксперт по оптимизации веб-контента для улучшения конверсии"},
		{Role: "user", Content: prompt},
	}

	req := ChatCompletionRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.7,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", OpenAIAPIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error: %s - %s", resp.Status, string(body))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, errors.New("no content was generated")
	}

	// Parse the content as JSON
	var improvements map[string]string
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &improvements); err != nil {
		// If parsing as JSON fails, return the content as is
		return map[string]string{
			"raw_content": result.Choices[0].Message.Content,
		}, nil
	}

	return improvements, nil
}

// GenerateCodeSnippet generates HTML code snippet for the improved content
func (c *OpenAIClient) GenerateCodeSnippet(originalHTML, improvementJSON string) (string, error) {
	prompt := fmt.Sprintf(`
На основе оригинального HTML и предложенных улучшений, создайте обновленный HTML-код.

Оригинальный HTML:
%s

Предложенные улучшения (JSON):
%s

Верните только HTML-код без объяснений.
`, originalHTML, improvementJSON)

	messages := []Message{
		{Role: "system", Content: "Вы - эксперт по HTML, CSS и веб-разработке"},
		{Role: "user", Content: prompt},
	}

	req := ChatCompletionRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", OpenAIAPIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error: %s - %s", resp.Status, string(body))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", errors.New("no content was generated")
	}

	return result.Choices[0].Message.Content, nil
}
