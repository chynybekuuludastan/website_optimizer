package prompts

import (
	"fmt"
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
)

// HeadingPrompt creates a prompt for optimizing page headings
func (g *Generator) HeadingPrompt(request *llm.ContentRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert in writing compelling page headings and titles.\n\n")
	sb.WriteString(fmt.Sprintf("Optimize the heading for the following website: %s\n\n", request.URL))

	// Add current heading
	sb.WriteString(fmt.Sprintf("Current heading: \"%s\"\n\n", request.Title))

	// Add SEO context if available
	if request.AnalysisResults != nil && len(request.AnalysisResults.SEO) > 0 {
		sb.WriteString("SEO Issues:\n")
		for _, issue := range prioritizeIssues(request.AnalysisResults.SEO, 3) {
			sb.WriteString(fmt.Sprintf("- %s\n", issue.Description))
		}
		sb.WriteString("\n")
	}

	// Target audience
	if request.TargetAudience != "" {
		sb.WriteString(fmt.Sprintf("Target Audience: %s\n\n", request.TargetAudience))
	}

	// Response instructions
	sb.WriteString("Create a compelling, concise heading that:\n")
	sb.WriteString("- Is attention-grabbing and emotionally resonant\n")
	sb.WriteString("- Contains relevant keywords naturally\n")
	sb.WriteString("- Is under 60 characters to prevent truncation in search results\n")
	sb.WriteString("- Clearly communicates the page's value proposition\n\n")

	sb.WriteString("Response format: JSON with a single field 'heading' containing your optimized heading.\n")
	sb.WriteString("Do not include any explanations, just return the JSON object.")

	return sb.String()
}

// MetaDescriptionPrompt creates a prompt for optimizing meta descriptions
func (g *Generator) MetaDescriptionPrompt(request *llm.ContentRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert in writing effective meta descriptions that improve click-through rates.\n\n")
	sb.WriteString(fmt.Sprintf("Create an optimized meta description for: %s\n\n", request.URL))

	// Add current content summary
	sb.WriteString(fmt.Sprintf("Page heading: \"%s\"\n\n", request.Title))

	// Add content excerpt
	contentExcerpt := request.Content
	if len(contentExcerpt) > 300 {
		contentExcerpt = contentExcerpt[:300] + "..."
	}
	sb.WriteString(fmt.Sprintf("Content excerpt: \"%s\"\n\n", contentExcerpt))

	// Response instructions
	sb.WriteString("Create a meta description that:\n")
	sb.WriteString("- Accurately summarizes the page content\n")
	sb.WriteString("- Includes a clear value proposition or call-to-action\n")
	sb.WriteString("- Contains relevant keywords naturally placed\n")
	sb.WriteString("- Is between 140-160 characters (optimal for search engines)\n")
	sb.WriteString("- Entices users to click through from search results\n\n")

	sb.WriteString("Response format: JSON with a single field 'meta_description' containing your optimized description.\n")
	sb.WriteString("Do not include any explanations, just return the JSON object.")

	return sb.String()
}

// CTAPrompt creates a prompt for optimizing call-to-action buttons and text
func (g *Generator) CTAPrompt(request *llm.ContentRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert in crafting high-converting call-to-action (CTA) elements.\n\n")
	sb.WriteString(fmt.Sprintf("Optimize the CTA for the following page: %s\n\n", request.URL))

	// Add current CTA and context
	sb.WriteString(fmt.Sprintf("Current CTA text: \"%s\"\n", request.CTAText))
	sb.WriteString(fmt.Sprintf("Page heading: \"%s\"\n\n", request.Title))

	// Target audience
	if request.TargetAudience != "" {
		sb.WriteString(fmt.Sprintf("Target Audience: %s\n\n", request.TargetAudience))
	}

	// Response instructions
	sb.WriteString("Create a compelling CTA that:\n")
	sb.WriteString("- Uses action-oriented language with strong verbs\n")
	sb.WriteString("- Creates a sense of urgency or value\n")
	sb.WriteString("- Is concise (typically 2-5 words)\n")
	sb.WriteString("- Clearly communicates what happens after clicking\n")
	sb.WriteString("- Aligns with the page's overall objective\n\n")

	sb.WriteString("Response format: JSON with a single field 'cta_button' containing your optimized CTA text.\n")
	sb.WriteString("Do not include any explanations, just return the JSON object.")

	return sb.String()
}

// ContentBlockPrompt creates a prompt for optimizing specific content blocks
func (g *Generator) ContentBlockPrompt(request *llm.ContentRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert content writer specializing in clear, engaging website copy.\n\n")
	sb.WriteString(fmt.Sprintf("Improve the content for the following page: %s\n\n", request.URL))

	// Add current content
	sb.WriteString("Current content:\n\n")
	sb.WriteString(request.Content)
	sb.WriteString("\n\n")

	// Add content issues if available
	if request.AnalysisResults != nil {
		if len(request.AnalysisResults.Content) > 0 {
			sb.WriteString("Content Issues:\n")
			for _, issue := range prioritizeIssues(request.AnalysisResults.Content, 3) {
				sb.WriteString(fmt.Sprintf("- %s\n", issue.Description))
			}
			sb.WriteString("\n")
		}

		if request.AnalysisResults.ReadabilityScore > 0 {
			sb.WriteString(fmt.Sprintf("Current readability score: %.1f (%s)\n\n",
				request.AnalysisResults.ReadabilityScore,
				request.AnalysisResults.ReadabilityLevel))
		}
	}

	// Target audience
	if request.TargetAudience != "" {
		sb.WriteString(fmt.Sprintf("Target Audience: %s\n\n", request.TargetAudience))
	}

	// Response instructions
	sb.WriteString("Improve this content by:\n")
	sb.WriteString("- Making it more engaging and concise\n")
	sb.WriteString("- Improving readability with shorter paragraphs and simpler language\n")
	sb.WriteString("- Adding subheadings where appropriate\n")
	sb.WriteString("- Naturally incorporating relevant keywords\n")
	sb.WriteString("- Maintaining the original meaning and key points\n\n")

	// Language instructions
	if request.Language != "" && request.Language != "en" {
		sb.WriteString(fmt.Sprintf("Provide the content in %s language.\n\n", request.Language))
	}

	sb.WriteString("Response format: JSON with a single field 'improved_content' containing your optimized content.\n")
	sb.WriteString("Do not include any explanations, just return the JSON object.")

	return sb.String()
}
