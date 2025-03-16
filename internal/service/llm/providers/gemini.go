package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm/prompts"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiProvider implements the Provider interface for Google's Gemini API
type GeminiProvider struct {
	apiKey    string
	modelName string
	client    *genai.Client
	logger    llm.Logger
	generator *prompts.Generator
}

// NewGeminiProvider creates a new Gemini provider using the official client
func NewGeminiProvider(apiKey string, modelName string, logger llm.Logger) (*GeminiProvider, error) {
	// Проверка параметров
	if apiKey == "" || apiKey == "YOUR_GEMINI_API_KEY" {
		return nil, errors.New("требуется действительный API ключ Gemini")
	}

	if modelName == "" {
		modelName = "gemini-1.5-flash" // Default model
	}

	if logger == nil {
		logger = &llm.DefaultLogger{}
	}

	// Initialize the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента Gemini: %w", err)
	}

	// Проверка валидности API ключа с простым запросом
	model := client.GenerativeModel("gemini-1.5-flash")
	_, err = model.GenerateContent(ctx, genai.Text("Test"))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("ошибка доступа к API Gemini: %w", err)
	}

	return &GeminiProvider{
		apiKey:    apiKey,
		modelName: modelName,
		client:    client,
		logger:    logger,
		generator: prompts.NewGenerator(),
	}, nil
}

// GetName returns the provider name
func (p *GeminiProvider) GetName() string {
	return "gemini"
}

// GenerateContent implements the Provider interface
func (p *GeminiProvider) GenerateContent(ctx context.Context, request *llm.ContentRequest) (*llm.ContentResponse, error) {
	startTime := time.Now()

	// Get the model
	model := p.client.GenerativeModel(p.modelName)

	// Configure the model settings
	model.SetTemperature(0.7)
	model.SetTopP(0.95)
	model.SetTopK(40)
	model.SetMaxOutputTokens(2048)

	// Generate prompt for content improvement
	prompt := p.generator.GenerateContentPrompt(request)

	p.logger.Debug("Sending prompt to Gemini", "prompt", prompt)

	// Определение параметров безопасности
	safetySettings := []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}

	// Применяем настройки безопасности
	model.SafetySettings = safetySettings

	// Generate content using the official client
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		p.logger.Error("Gemini API error", "error", err)
		return nil, fmt.Errorf("ошибка Gemini API: %w", err)
	}

	// Process the response
	if len(resp.Candidates) == 0 {
		return nil, errors.New("контент не сгенерирован")
	}

	// Check for safety issues
	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != genai.BlockReasonUnspecified {
		return nil, fmt.Errorf("контент заблокирован по причине: %s", resp.PromptFeedback.BlockReason)
	}

	// Extract text from response
	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			responseText += string(textPart)
		}
	}

	p.logger.Debug("Received response from Gemini", "response", responseText)

	// Try to parse the response as JSON
	var improvements map[string]string
	if err := json.Unmarshal([]byte(responseText), &improvements); err != nil {
		p.logger.Error("Failed to parse Gemini response as JSON",
			"error", err,
			"content", responseText)

		// Try to extract content using regex as fallback
		parsed, extractErr := llm.ExtractContentFromText(responseText)
		if extractErr != nil {
			return nil, fmt.Errorf("ошибка разбора ответа: %w, оригинальная ошибка: %w", extractErr, err)
		}

		p.logger.Info("Extracted content using regex", "parsed", parsed)
		improvements = parsed
	}

	// Validate that we have all required fields
	if improvements["heading"] == "" || improvements["cta_button"] == "" || improvements["improved_content"] == "" {
		p.logger.Info("Incomplete response from Gemini", "improvements", improvements)
	}

	// Create the response
	contentResponse := &llm.ContentResponse{
		Title:          improvements["heading"],
		CTAText:        improvements["cta_button"],
		Content:        improvements["improved_content"],
		ProviderUsed:   "gemini",
		ProcessingTime: time.Since(startTime),
	}

	return contentResponse, nil
}

// GenerateHTML implements the Provider interface
func (p *GeminiProvider) GenerateHTML(ctx context.Context, originalContent string, improved *llm.ContentResponse) (string, error) {
	// Get the model
	model := p.client.GenerativeModel(p.modelName)

	// Configure the model for code generation
	model.SetTemperature(0.2)
	model.SetMaxOutputTokens(2048)

	// Generate prompt for HTML generation
	prompt := p.generator.GenerateHTMLPrompt(originalContent, improved)

	p.logger.Debug("Sending HTML prompt to Gemini", "prompt", prompt)

	// Generate HTML
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		p.logger.Error("Gemini HTML generation error", "error", err)
		return "", fmt.Errorf("ошибка генерации HTML через Gemini: %w", err)
	}

	// Process the response
	if len(resp.Candidates) == 0 {
		return "", errors.New("HTML не сгенерирован")
	}

	// Extract text from response
	var html string
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			html += string(textPart)
		}
	}

	p.logger.Debug("Received HTML from Gemini", "html", html)

	// Remove any markdown code blocks
	html = llm.CleanCodeBlocks(html)

	// If the HTML is empty, generate a basic version
	if html == "" {
		html = fmt.Sprintf("<h1>%s</h1>\n<p>%s</p>\n<button>%s</button>",
			improved.Title, improved.Content, improved.CTAText)
	}

	return html, nil
}

// Close closes the Gemini client
func (p *GeminiProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
