package parser

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

// WebsiteData represents the data extracted from a website
type WebsiteData struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	H1          []string          `json:"h1"`
	H2          []string          `json:"h2"`
	H3          []string          `json:"h3"`
	MetaTags    map[string]string `json:"meta_tags"`
	Links       []Link            `json:"links"`
	Images      []Image           `json:"images"`
	Scripts     []Script          `json:"scripts"`
	Styles      []Style           `json:"styles"`
	HTML        string            `json:"html"`
	StatusCode  int               `json:"status_code"`
	LoadTime    time.Duration     `json:"load_time"`
	TextContent string            `json:"text_content"`
}

// Link represents a hyperlink on the page
type Link struct {
	URL        string `json:"url"`
	Text       string `json:"text"`
	IsInternal bool   `json:"is_internal"`
	StatusCode int    `json:"status_code"`
}

// Image represents an image on the page
type Image struct {
	URL      string `json:"url"`
	Alt      string `json:"alt"`
	Width    string `json:"width"`
	Height   string `json:"height"`
	FileSize int64  `json:"file_size"`
}

// Script represents a JavaScript file on the page
type Script struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Style represents a CSS file on the page
type Style struct {
	URL string `json:"url"`
}

// ParseWebsite parses a website and returns structured data
func ParseWebsite(targetURL string, timeout time.Duration) (*WebsiteData, error) {
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	websiteData := &WebsiteData{
		URL:      targetURL,
		MetaTags: make(map[string]string),
	}

	c := colly.NewCollector(
		colly.AllowedDomains(parsedURL.Hostname()),
		colly.MaxDepth(1),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(timeout)

	startTime := time.Now()

	// On every <a> element
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := Link{
			URL:  e.Attr("href"),
			Text: strings.TrimSpace(e.Text),
		}

		absoluteURL := e.Request.AbsoluteURL(link.URL)
		if absoluteURL != "" {
			link.URL = absoluteURL
		}

		linkURL, err := url.Parse(link.URL)
		if err == nil && linkURL.Hostname() == parsedURL.Hostname() {
			link.IsInternal = true
		}

		websiteData.Links = append(websiteData.Links, link)
	})

	// On every <img> element
	c.OnHTML("img[src]", func(e *colly.HTMLElement) {
		img := Image{
			URL:    e.Request.AbsoluteURL(e.Attr("src")),
			Alt:    e.Attr("alt"),
			Width:  e.Attr("width"),
			Height: e.Attr("height"),
		}
		websiteData.Images = append(websiteData.Images, img)
	})

	// On every <script> element
	c.OnHTML("script[src]", func(e *colly.HTMLElement) {
		script := Script{
			URL:  e.Request.AbsoluteURL(e.Attr("src")),
			Type: e.Attr("type"),
		}
		websiteData.Scripts = append(websiteData.Scripts, script)
	})

	// On every <link rel="stylesheet"> element
	c.OnHTML("link[rel='stylesheet'][href]", func(e *colly.HTMLElement) {
		style := Style{
			URL: e.Request.AbsoluteURL(e.Attr("href")),
		}
		websiteData.Styles = append(websiteData.Styles, style)
	})

	// Extract meta tags, title, and headings
	c.OnHTML("html", func(e *colly.HTMLElement) {
		// Extract title
		websiteData.Title = e.ChildText("title")

		// Extract meta description
		e.ForEach("meta", func(_ int, el *colly.HTMLElement) {
			name := el.Attr("name")
			content := el.Attr("content")
			if name != "" && content != "" {
				websiteData.MetaTags[name] = content
			}
			if name == "description" {
				websiteData.Description = content
			}
		})

		// Extract headings
		e.ForEach("h1", func(_ int, el *colly.HTMLElement) {
			websiteData.H1 = append(websiteData.H1, strings.TrimSpace(el.Text))
		})
		e.ForEach("h2", func(_ int, el *colly.HTMLElement) {
			websiteData.H2 = append(websiteData.H2, strings.TrimSpace(el.Text))
		})
		e.ForEach("h3", func(_ int, el *colly.HTMLElement) {
			websiteData.H3 = append(websiteData.H3, strings.TrimSpace(el.Text))
		})

		// Get main content text (simplified)
		doc := e.DOM
		// Remove script and style elements
		doc.Find("script, style").Remove()
		websiteData.TextContent = strings.TrimSpace(doc.Text())

		// Save HTML
		html, err := doc.Html()
		if err == nil {
			websiteData.HTML = html
		}
	})

	// Set up on response callback
	c.OnResponse(func(r *colly.Response) {
		websiteData.StatusCode = r.StatusCode
		websiteData.LoadTime = time.Since(startTime)
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		websiteData.StatusCode = r.StatusCode
		if r.StatusCode == 0 {
			websiteData.StatusCode = http.StatusInternalServerError
		}
	})

	// Start scraping
	err = c.Visit(targetURL)
	if err != nil {
		return websiteData, err
	}

	// Wait for scraping to finish
	c.Wait()

	// Check links status
	if len(websiteData.Links) > 0 {
		checkLinksStatus(websiteData, timeout)
	}

	return websiteData, nil
}

// checkLinksStatus checks the HTTP status of each link
func checkLinksStatus(data *WebsiteData, timeout time.Duration) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	for i, link := range data.Links {
		if !link.IsInternal {
			continue // Skip external links to avoid too many requests
		}

		req, err := http.NewRequest("HEAD", link.URL, nil)
		if err != nil {
			data.Links[i].StatusCode = http.StatusInternalServerError
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebsiteAnalyzer/1.0)")

		resp, err := client.Do(req)
		if err != nil {
			data.Links[i].StatusCode = http.StatusInternalServerError
			continue
		}

		data.Links[i].StatusCode = resp.StatusCode
		resp.Body.Close()
	}
}

// AnalyzeHTMLValidity checks for HTML validity issues
func AnalyzeHTMLValidity(html string) map[string]interface{} {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		}
	}

	issues := []map[string]string{}

	// Check for semantic tags
	semanticTags := map[string]int{
		"header":  0,
		"footer":  0,
		"nav":     0,
		"main":    0,
		"article": 0,
		"section": 0,
		"aside":   0,
	}

	for tag := range semanticTags {
		count := doc.Find(tag).Length()
		semanticTags[tag] = count
	}

	// Check for image alt attributes
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		alt, exists := s.Attr("alt")
		if !exists || alt == "" {
			src, _ := s.Attr("src")
			issues = append(issues, map[string]string{
				"type":     "missing_alt",
				"element":  "img",
				"location": src,
			})
		}
	})

	// Check for heading structure
	headingLevels := []int{0, 0, 0, 0, 0, 0}
	headingsInOrder := true

	for i := 1; i <= 6; i++ {
		selector := "h" + string(rune('0'+i))
		headingLevels[i-1] = doc.Find(selector).Length()
	}

	// Check if headings are used in order (simplified)
	if headingLevels[0] == 0 && (headingLevels[1] > 0 || headingLevels[2] > 0) {
		headingsInOrder = false
		issues = append(issues, map[string]string{
			"type":     "heading_order",
			"element":  "h2/h3 without h1",
			"location": "document",
		})
	}

	return map[string]interface{}{
		"valid":           len(issues) == 0,
		"issues":          issues,
		"semanticTags":    semanticTags,
		"headingLevels":   headingLevels,
		"headingsInOrder": headingsInOrder,
	}
}
