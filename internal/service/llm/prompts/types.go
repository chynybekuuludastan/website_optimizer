package prompts

type PromptType string

const (
	PromptTypeHeading     PromptType = "heading"
	PromptTypeMeta        PromptType = "meta"
	PromptTypeContent     PromptType = "content"
	PromptTypeCTA         PromptType = "cta"
	PromptTypeHTML        PromptType = "html"
	PromptTypeFullContent PromptType = "full_content" // Current default
)
