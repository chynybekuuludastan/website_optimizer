package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm/prompts"
)

const (
	openAICompletionsURL = "https://api.openai.com/v1/chat/completions"
	defaultTimeout       = 30 * time.Second
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
	logger     llm.Logger
	generator  *prompts.Generator
}

// OpenAIMessage represents a message in the OpenAI chat API
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIRequest represents a request to OpenAI's chat completions API
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

// OpenAIResponse represents the response from OpenAI's chat completions API
type OpenAIResponse struct {
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

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, model string, logger llm.Logger) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	if model == "" {
		model = "gpt-4" // Default model
	}

	if logger == nil {
		logger = &llm.DefaultLogger{}
	}

	return &OpenAIProvider{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: defaultTimeout},
		logger:     logger,
		generator:  prompts.NewGenerator(),
	}, nil
}

// GetName returns the provider name
func (p *OpenAIProvider) GetName() string {
	return "openai"
}

// GenerateContent implements the Provider interface
func (p *OpenAIProvider) GenerateContent(ctx context.Context, request *llm.ContentRequest) (*llm.ContentResponse, error) {
	// Generate prompt
	prompt := p.generator.GenerateContentPrompt(request)

	messages := []OpenAIMessage{
		{
			Role:    "system",
			Content: "You are an expert in web content optimization to improve conversions and user experience.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	apiRequest := OpenAIRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	apiResponse, err := p.makeRequest(ctx, apiRequest)
	if err != nil {
		return nil, err
	}

	if len(apiResponse.Choices) == 0 {
		return nil, errors.New("empty response from OpenAI")
	}

	responseContent := apiResponse.Choices[0].Message.Content

	// Parse the JSON response
	var improvements map[string]string
	if err := json.Unmarshal([]byte(responseContent), &improvements); err != nil {
		p.logger.Error("Failed to parse OpenAI response as JSON",
			"error", err,
			"content", responseContent)

		// Try to extract content using regex as fallback
		parsed, extractErr := llm.ExtractContentFromText(responseContent)
		if extractErr != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		improvements = parsed
	}

	return &llm.ContentResponse{
		Title:        improvements["heading"],
		CTAText:      improvements["cta_button"],
		Content:      improvements["improved_content"],
		ProviderUsed: p.GetName(),
	}, nil
}

// GenerateHTML implements the Provider interface
func (p *OpenAIProvider) GenerateHTML(ctx context.Context, originalContent string, improved *llm.ContentResponse) (string, error) {
	// Generate prompt for HTML
	prompt := p.generator.GenerateHTMLPrompt(originalContent, improved)

	messages := []OpenAIMessage{
		{
			Role:    "system",
			Content: "You are an expert HTML developer. Generate clean, semantic HTML code.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	apiRequest := OpenAIRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: 0.3, // Lower temperature for more deterministic output
	}

	apiResponse, err := p.makeRequest(ctx, apiRequest)
	if err != nil {
		return "", err
	}

	if len(apiResponse.Choices) == 0 {
		return "", errors.New("empty response from OpenAI")
	}

	html := apiResponse.Choices[0].Message.Content

	// Remove markdown code blocks if present
	html = llm.CleanCodeBlocks(html)

	return html, nil
}

// makeRequest sends a request to the OpenAI API
func (p *OpenAIProvider) makeRequest(ctx context.Context, request OpenAIRequest) (*OpenAIResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openAICompletionsURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		p.logger.Error("OpenAI API error",
			"status", resp.Status,
			"body", string(body))
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var apiResponse OpenAIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &apiResponse, nil
}

// Close implements the Provider interface
func (p *OpenAIProvider) Close() error {
	// Nothing to close for HTTP client
	return nil
}
