package providers

import (
	"context"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
)

// Provider defines the interface that all LLM providers must implement
type Provider interface {
	// GenerateContent generates improved content based on the request
	GenerateContent(ctx context.Context, request *llm.ContentRequest) (*llm.ContentResponse, error)

	// GenerateHTML generates HTML code for the improved content
	GenerateHTML(ctx context.Context, original string, improved *llm.ContentResponse) (string, error)

	// GetName returns the name of the provider
	GetName() string

	// Close closes any connections or resources used by the provider
	Close() error
}
