package analyzer

import (
	"strings"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

// SEOResult contains the SEO analysis results
type SEOResult struct {
	Score            float64                  `json:"score"`
	MissingMetaTitle bool                     `json:"missing_meta_title"`
	MetaTitleLength  int                      `json:"meta_title_length"`
	MissingMetaDesc  bool                     `json:"missing_meta_description"`
	MetaDescLength   int                      `json:"meta_description_length"`
	HeadingStructure map[string]int           `json:"heading_structure"`
	MissingAltTags   []string                 `json:"missing_alt_tags"`
	KeywordDensity   map[string]float64       `json:"keyword_density"`
	InternalLinks    int                      `json:"internal_links"`
	ExternalLinks    int                      `json:"external_links"`
	BrokenLinks      []string                 `json:"broken_links"`
	CanonicalURL     string                   `json:"canonical_url"`
	MobileFriendly   bool                     `json:"mobile_friendly"`
	Issues           []map[string]interface{} `json:"issues"`
	Recommendations  []string                 `json:"recommendations"`
}

// AnalyzeSEO performs SEO analysis on the website data
func AnalyzeSEO(data *parser.WebsiteData) map[string]interface{} {
	result := SEOResult{
		HeadingStructure: make(map[string]int),
		KeywordDensity:   make(map[string]float64),
		MissingAltTags:   []string{},
		BrokenLinks:      []string{},
		Issues:           []map[string]interface{}{},
		Recommendations:  []string{},
	}

	// Check meta title
	result.MissingMetaTitle = data.Title == ""
	result.MetaTitleLength = len(data.Title)
	if result.MissingMetaTitle {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_title",
			"severity":    "high",
			"description": "The page is missing a title tag",
		})
		result.Recommendations = append(result.Recommendations, "Add a descriptive title tag to the page")
	} else if result.MetaTitleLength < 30 || result.MetaTitleLength > 60 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "title_length",
			"severity":    "medium",
			"description": "The title tag is either too short or too long",
			"current":     result.MetaTitleLength,
			"recommended": "30-60 characters",
		})
		if result.MetaTitleLength < 30 {
			result.Recommendations = append(result.Recommendations, "Make the title tag more descriptive (at least 30 characters)")
		} else {
			result.Recommendations = append(result.Recommendations, "Shorten the title tag (maximum 60 characters recommended)")
		}
	}

	// Check meta description
	metaDesc := ""
	if desc, ok := data.MetaTags["description"]; ok {
		metaDesc = desc
	}
	result.MissingMetaDesc = metaDesc == ""
	result.MetaDescLength = len(metaDesc)
	if result.MissingMetaDesc {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_description",
			"severity":    "high",
			"description": "The page is missing a meta description",
		})
		result.Recommendations = append(result.Recommendations, "Add a meta description tag with a concise summary of the page content")
	} else if result.MetaDescLength < 50 || result.MetaDescLength > 160 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "description_length",
			"severity":    "medium",
			"description": "The meta description is either too short or too long",
			"current":     result.MetaDescLength,
			"recommended": "50-160 characters",
		})
		if result.MetaDescLength < 50 {
			result.Recommendations = append(result.Recommendations, "Make the meta description more descriptive (at least 50 characters)")
		} else {
			result.Recommendations = append(result.Recommendations, "Shorten the meta description (maximum 160 characters recommended)")
		}
	}

	// Analyze heading structure
	result.HeadingStructure["h1"] = len(data.H1)
	result.HeadingStructure["h2"] = len(data.H2)
	result.HeadingStructure["h3"] = len(data.H3)

	if len(data.H1) == 0 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_h1",
			"severity":    "high",
			"description": "The page is missing an H1 heading",
		})
		result.Recommendations = append(result.Recommendations, "Add an H1 heading that clearly describes the page content")
	} else if len(data.H1) > 1 {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "multiple_h1",
			"severity":    "medium",
			"description": "The page has multiple H1 headings",
			"count":       len(data.H1),
		})
		result.Recommendations = append(result.Recommendations, "Use only one H1 heading per page")
	}

	// Check image alt tags
	for _, img := range data.Images {
		if img.Alt == "" {
			result.MissingAltTags = append(result.MissingAltTags, img.URL)
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "missing_alt",
				"severity":    "medium",
				"description": "An image is missing an alt attribute",
				"url":         img.URL,
			})
		}
	}
	if len(result.MissingAltTags) > 0 {
		result.Recommendations = append(result.Recommendations, "Add descriptive alt text to all images")
	}

	// Analyze links
	for _, link := range data.Links {
		if link.IsInternal {
			result.InternalLinks++
		} else {
			result.ExternalLinks++
		}

		if link.StatusCode >= 400 {
			result.BrokenLinks = append(result.BrokenLinks, link.URL)
			result.Issues = append(result.Issues, map[string]interface{}{
				"type":        "broken_link",
				"severity":    "high",
				"description": "A link on the page is broken",
				"url":         link.URL,
				"status_code": link.StatusCode,
			})
		}
	}
	if len(result.BrokenLinks) > 0 {
		result.Recommendations = append(result.Recommendations, "Fix broken links on the page")
	}

	// Look for canonical URL
	for key, value := range data.MetaTags {
		if key == "canonical" {
			result.CanonicalURL = value
			break
		}
	}
	if result.CanonicalURL == "" {
		result.Issues = append(result.Issues, map[string]interface{}{
			"type":        "missing_canonical",
			"severity":    "low",
			"description": "The page is missing a canonical URL",
		})
		result.Recommendations = append(result.Recommendations, "Add a canonical URL to prevent duplicate content issues")
	}

	// Simple keyword density analysis
	if data.TextContent != "" {
		words := strings.Fields(strings.ToLower(data.TextContent))
		wordCount := make(map[string]int)
		totalWords := len(words)

		for _, word := range words {
			// Skip short words and common stop words
			if len(word) <= 2 || isStopWord(word) {
				continue
			}
			// Remove punctuation
			word = strings.Trim(word, ".,?!:;()")
			if word != "" {
				wordCount[word]++
			}
		}

		// Calculate keyword density
		for word, count := range wordCount {
			if count >= 3 { // Only include words that appear at least 3 times
				result.KeywordDensity[word] = float64(count) / float64(totalWords) * 100
			}
		}
	}

	// Calculate overall score based on issues
	// Simple scoring: start with 100 and subtract points for issues
	score := 100.0
	for _, issue := range result.Issues {
		severity := issue["severity"].(string)
		switch severity {
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}
	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	result.Score = score

	return map[string]interface{}{
		"score":                    result.Score,
		"missing_meta_title":       result.MissingMetaTitle,
		"meta_title_length":        result.MetaTitleLength,
		"missing_meta_description": result.MissingMetaDesc,
		"meta_description_length":  result.MetaDescLength,
		"heading_structure":        result.HeadingStructure,
		"missing_alt_tags":         result.MissingAltTags,
		"keyword_density":          result.KeywordDensity,
		"internal_links":           result.InternalLinks,
		"external_links":           result.ExternalLinks,
		"broken_links":             result.BrokenLinks,
		"canonical_url":            result.CanonicalURL,
		"issues":                   result.Issues,
		"recommendations":          result.Recommendations,
	}
}

// isStopWord checks if a word is a common stop word
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"а": true, "без": true, "более": true, "бы": true, "был": true, "была": true, "были": true, "было": true,
		"быть": true, "в": true, "вам": true, "вас": true, "весь": true, "во": true, "вот": true, "все": true,
		"всего": true, "всех": true, "вы": true, "где": true, "да": true, "даже": true, "для": true, "до": true,
		"его": true, "ее": true, "ей": true, "ему": true, "если": true, "есть": true, "еще": true, "же": true,
		"за": true, "здесь": true, "и": true, "из": true, "или": true, "им": true, "их": true, "к": true,
		"как": true, "ко": true, "когда": true, "кто": true, "ли": true, "либо": true, "мне": true, "может": true,
		"мы": true, "на": true, "надо": true, "наш": true, "не": true, "него": true, "нее": true, "нет": true,
		"ни": true, "них": true, "но": true, "ну": true, "о": true, "об": true, "однако": true, "он": true,
		"она": true, "они": true, "оно": true, "от": true, "очень": true, "по": true, "под": true, "при": true,
		"с": true, "со": true, "так": true, "также": true, "такой": true, "там": true, "те": true, "тем": true,
		"то": true, "того": true, "тоже": true, "той": true, "только": true, "том": true, "ты": true, "у": true,
		"уже": true, "хотя": true, "чего": true, "чей": true, "чем": true, "что": true, "чтобы": true, "эта": true,
		"эти": true, "это": true, "я": true,
		"the": true, "of": true, "and": true, "a": true, "to": true, "in": true, "is": true, "you": true,
		"that": true, "it": true, "he": true, "was": true, "for": true, "on": true, "are": true, "as": true,
		"with": true, "his": true, "they": true, "I": true, "at": true, "be": true, "this": true, "have": true,
		"from": true, "or": true, "one": true, "had": true, "by": true, "but": true, "not": true, "what": true,
		"all": true, "were": true, "we": true, "when": true, "your": true, "can": true, "said": true, "there": true,
		"use": true, "an": true, "each": true, "which": true, "she": true, "do": true, "how": true, "their": true,
		"if": true, "will": true, "up": true, "other": true, "about": true, "out": true, "many": true, "then": true,
		"them": true, "these": true, "so": true, "some": true, "her": true, "would": true, "make": true, "like": true,
		"him": true, "into": true, "time": true, "has": true, "look": true, "two": true, "more": true, "go": true,
		"see": true, "no": true, "way": true, "could": true, "my": true, "than": true, "been": true, "who": true,
		"its": true, "now": true, "did": true, "get": true, "come": true,
	}
	return stopWords[word]
}
