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

// WebsiteData contains all the extracted information from a website
type WebsiteData struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Keywords    string            `json:"keywords"`
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
	NoFollow   bool   `json:"no_follow"`
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
	URL        string `json:"url"`
	Type       string `json:"type"`
	IsAsync    bool   `json:"is_async"`
	IsDeferred bool   `json:"is_deferred"`
}

// Style represents a CSS file on the page
type Style struct {
	URL    string `json:"url"`
	Media  string `json:"media"`
	IsLink bool   `json:"is_link"`
}

// ParseOptions allows customizing the parsing behavior
type ParseOptions struct {
	Timeout           time.Duration
	MaxDepth          int
	FollowRedirects   bool
	CheckExternalURLs bool
	UserAgent         string
}

// DefaultParseOptions returns the default parsing options
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		Timeout:           30 * time.Second,
		MaxDepth:          1,
		FollowRedirects:   true,
		CheckExternalURLs: false,
		UserAgent:         "Mozilla/5.0 (compatible; WebsiteParser/1.0)",
	}
}

// ParseWebsite parses a website and returns structured data
func ParseWebsite(targetURL string, options ...ParseOptions) (*WebsiteData, error) {
	// Use default options if not provided
	opts := DefaultParseOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	// Add scheme if missing
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Initialize the data structure
	websiteData := &WebsiteData{
		URL:      targetURL,
		MetaTags: make(map[string]string),
	}

	// Set up the collector
	c := colly.NewCollector(
		colly.AllowedDomains(parsedURL.Hostname()),
		colly.MaxDepth(opts.MaxDepth),
		colly.Async(true),
	)

	// Set rate limiting to be respectful
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	// Set user agent
	if opts.UserAgent != "" {
		c.UserAgent = opts.UserAgent
	} else {
		extensions.RandomUserAgent(c)
	}

	c.SetRequestTimeout(opts.Timeout)

	// Track the start time for load time calculation
	startTime := time.Now()

	// Process hyperlinks
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href == "" || href == "#" || strings.HasPrefix(href, "javascript:") {
			return
		}

		link := Link{
			URL:      href,
			Text:     strings.TrimSpace(e.Text),
			NoFollow: e.Attr("rel") == "nofollow",
		}

		// Convert to absolute URL
		absoluteURL := e.Request.AbsoluteURL(link.URL)
		if absoluteURL != "" {
			link.URL = absoluteURL
		}

		// Determine if link is internal
		linkURL, err := url.Parse(link.URL)
		if err == nil && linkURL.Hostname() == parsedURL.Hostname() {
			link.IsInternal = true
		}

		websiteData.Links = append(websiteData.Links, link)
	})

	// Process images
	c.OnHTML("img[src]", func(e *colly.HTMLElement) {
		img := Image{
			URL:    e.Request.AbsoluteURL(e.Attr("src")),
			Alt:    e.Attr("alt"),
			Width:  e.Attr("width"),
			Height: e.Attr("height"),
		}
		websiteData.Images = append(websiteData.Images, img)
	})

	// Process scripts
	c.OnHTML("script[src]", func(e *colly.HTMLElement) {
		script := Script{
			URL:        e.Request.AbsoluteURL(e.Attr("src")),
			Type:       e.Attr("type"),
			IsAsync:    e.Attr("async") != "",
			IsDeferred: e.Attr("defer") != "",
		}
		websiteData.Scripts = append(websiteData.Scripts, script)
	})

	// Process stylesheets
	c.OnHTML("link[rel='stylesheet'][href]", func(e *colly.HTMLElement) {
		style := Style{
			URL:    e.Request.AbsoluteURL(e.Attr("href")),
			Media:  e.Attr("media"),
			IsLink: true,
		}
		websiteData.Styles = append(websiteData.Styles, style)
	})

	// Process inline styles
	c.OnHTML("style", func(e *colly.HTMLElement) {
		style := Style{
			URL:    "",
			Media:  e.Attr("media"),
			IsLink: false,
		}
		websiteData.Styles = append(websiteData.Styles, style)
	})

	// Extract metadata, headings, and text content
	c.OnHTML("html", func(e *colly.HTMLElement) {
		// Extract title
		websiteData.Title = e.ChildText("title")

		// Extract all meta tags
		e.ForEach("meta", func(_ int, el *colly.HTMLElement) {
			name := el.Attr("name")
			if name == "" {
				name = el.Attr("property")
			}
			content := el.Attr("content")

			if name != "" && content != "" {
				websiteData.MetaTags[name] = content

				// Extract specific meta tags
				switch name {
				case "description":
					websiteData.Description = content
				case "keywords":
					websiteData.Keywords = content
				}
			}
		})

		// Extract headings
		e.ForEach("h1", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				websiteData.H1 = append(websiteData.H1, text)
			}
		})
		e.ForEach("h2", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				websiteData.H2 = append(websiteData.H2, text)
			}
		})
		e.ForEach("h3", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				websiteData.H3 = append(websiteData.H3, text)
			}
		})

		// Extract text content
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

	// Handle response
	c.OnResponse(func(r *colly.Response) {
		websiteData.StatusCode = r.StatusCode
		websiteData.LoadTime = time.Since(startTime)
	})

	// Handle errors
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

	// Wait for all goroutines to finish
	c.Wait()

	// Check link statuses after initial parsing
	if len(websiteData.Links) > 0 {
		checkLinksStatus(websiteData, opts.Timeout, opts.CheckExternalURLs)
	}

	// Estimate image file sizes
	estimateImageSizes(websiteData, opts.Timeout)

	return websiteData, nil
}

// checkLinksStatus checks the HTTP status of links
func checkLinksStatus(data *WebsiteData, timeout time.Duration, checkExternal bool) {
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
		// Skip external links unless specifically requested
		if !link.IsInternal && !checkExternal {
			continue
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

// estimateImageSizes tries to get the file size of images
func estimateImageSizes(data *WebsiteData, timeout time.Duration) {
	client := &http.Client{
		Timeout: timeout,
	}

	for i, img := range data.Images {
		req, err := http.NewRequest("HEAD", img.URL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebsiteAnalyzer/1.0)")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			data.Images[i].FileSize = resp.ContentLength
		}
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

	// Check if headings are used in order
	if headingLevels[0] == 0 && (headingLevels[1] > 0 || headingLevels[2] > 0) {
		headingsInOrder = false
		issues = append(issues, map[string]string{
			"type":     "heading_order",
			"element":  "h2/h3 without h1",
			"location": "document",
		})
	}

	// Check for forms with required fields but no labels
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		requiredInputs := s.Find("input[required], select[required], textarea[required]")
		requiredInputs.Each(func(j int, input *goquery.Selection) {
			id, hasID := input.Attr("id")
			if hasID {
				label := doc.Find("label[for='" + id + "']")
				if label.Length() == 0 {
					issues = append(issues, map[string]string{
						"type":     "missing_label",
						"element":  "required input",
						"location": id,
					})
				}
			}
		})
	})

	return map[string]interface{}{
		"valid":           len(issues) == 0,
		"issues":          issues,
		"semanticTags":    semanticTags,
		"headingLevels":   headingLevels,
		"headingsInOrder": headingsInOrder,
	}
}

// GetBrokenLinks returns a list of broken links from the website data
func GetBrokenLinks(data *WebsiteData) []Link {
	brokenLinks := []Link{}
	for _, link := range data.Links {
		if link.StatusCode >= 400 {
			brokenLinks = append(brokenLinks, link)
		}
	}
	return brokenLinks
}

// GetMissingAltImages returns a list of images missing alt attributes
func GetMissingAltImages(data *WebsiteData) []Image {
	missingAlt := []Image{}
	for _, img := range data.Images {
		if img.Alt == "" {
			missingAlt = append(missingAlt, img)
		}
	}
	return missingAlt
}

// GetRenderBlockingResources returns scripts and styles that may block rendering
func GetRenderBlockingResources(data *WebsiteData) map[string][]string {
	blocking := map[string][]string{
		"scripts": {},
		"styles":  {},
	}

	for _, script := range data.Scripts {
		if !script.IsAsync && !script.IsDeferred {
			blocking["scripts"] = append(blocking["scripts"], script.URL)
		}
	}

	for _, style := range data.Styles {
		if style.IsLink {
			blocking["styles"] = append(blocking["styles"], style.URL)
		}
	}

	return blocking
}
