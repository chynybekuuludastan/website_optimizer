package parser

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

// WebsiteData contains all the extracted information from a website
type WebsiteData struct {
	URL             string            `json:"url"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Keywords        string            `json:"keywords"`
	H1              []string          `json:"h1"`
	H2              []string          `json:"h2"`
	H3              []string          `json:"h3"`
	MetaTags        map[string]string `json:"meta_tags"`
	Links           []Link            `json:"links"`
	Images          []Image           `json:"images"`
	Scripts         []Script          `json:"scripts"`
	Styles          []Style           `json:"styles"`
	HTML            string            `json:"html"`
	StatusCode      int               `json:"status_code"`
	LoadTime        time.Duration     `json:"load_time"`
	TextContent     string            `json:"text_content"`
	Screenshots     map[string][]byte `json:"screenshots,omitempty"`
	Technologies    []Technology      `json:"technologies,omitempty"`
	JavaScriptError string            `json:"javascript_error,omitempty"`
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

// Technology represents a detected technology on the website
type Technology struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Version     string   `json:"version,omitempty"`
	Confidence  int      `json:"confidence"`
	Categories  []string `json:"categories,omitempty"`
	Description string   `json:"description,omitempty"`
	Website     string   `json:"website,omitempty"`
}

// DeviceConfig represents configuration for a specific device emulation
type DeviceConfig struct {
	Name              string  `json:"name"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"device_scale_factor"`
	Mobile            bool    `json:"mobile"`
	UserAgent         string  `json:"user_agent"`
}

// ParseOptions allows customizing the parsing behavior
type ParseOptions struct {
	Timeout            time.Duration
	MaxDepth           int
	FollowRedirects    bool
	CheckExternalURLs  bool
	UserAgent          string
	UseHeadlessBrowser bool
	ExecuteJavaScript  bool
	JavaScriptTimeout  time.Duration
	MaxRetries         int
	RetryDelay         time.Duration
	Concurrency        int
	RespectRobotsTxt   bool
	CaptureScreenshots bool
	ScreenshotDevices  []DeviceConfig
	DetectTechnologies bool
	BypassAntiBot      bool
	WaitForSelector    string
	WaitTime           time.Duration
	ProxyURL           string
	Headers            map[string]string
	Cookies            []*http.Cookie
	CustomChromePath   string
}

// DefaultParseOptions returns the default parsing options
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		Timeout:            30 * time.Second,
		MaxDepth:           1,
		FollowRedirects:    true,
		CheckExternalURLs:  false,
		UserAgent:          "Mozilla/5.0 (compatible; WebsiteParser/1.0)",
		UseHeadlessBrowser: false,
		ExecuteJavaScript:  false,
		JavaScriptTimeout:  20 * time.Second,
		MaxRetries:         3,
		RetryDelay:         2 * time.Second,
		Concurrency:        2,
		RespectRobotsTxt:   true,
		CaptureScreenshots: false,
		ScreenshotDevices:  []DeviceConfig{},
		DetectTechnologies: false,
		BypassAntiBot:      false,
		WaitForSelector:    "",
		WaitTime:           0,
		Headers:            map[string]string{},
		Cookies:            []*http.Cookie{},
	}
}

// Standard device configurations
var (
	DesktopDevice = DeviceConfig{
		Name:              "desktop",
		Width:             1920,
		Height:            1080,
		DeviceScaleFactor: 1.0,
		Mobile:            false,
		UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}

	MobileDevice = DeviceConfig{
		Name:              "mobile",
		Width:             375,
		Height:            812,
		DeviceScaleFactor: 3.0,
		Mobile:            true,
		UserAgent:         "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
	}

	TabletDevice = DeviceConfig{
		Name:              "tablet",
		Width:             768,
		Height:            1024,
		DeviceScaleFactor: 2.0,
		Mobile:            true,
		UserAgent:         "Mozilla/5.0 (iPad; CPU OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
	}
)

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
		URL:         targetURL,
		MetaTags:    make(map[string]string),
		Screenshots: make(map[string][]byte),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Track the start time for load time calculation
	startTime := time.Now()

	// Choose parsing method based on options
	var parseErr error

	// If using headless browser for JavaScript or screenshots
	if opts.UseHeadlessBrowser || opts.ExecuteJavaScript || opts.CaptureScreenshots {
		parseErr = parseWithHeadlessBrowser(ctx, websiteData, targetURL, opts)

		// If headless browser fails and it's optional, fall back to standard parsing
		if parseErr != nil && !opts.UseHeadlessBrowser {
			websiteData.JavaScriptError = parseErr.Error()
			parseErr = parseWithColly(ctx, websiteData, parsedURL, opts)
		}
	} else {
		// Standard parsing without JavaScript execution
		parseErr = parseWithColly(ctx, websiteData, parsedURL, opts)
	}

	// Calculate load time
	websiteData.LoadTime = time.Since(startTime)

	// Detect technologies if requested
	if opts.DetectTechnologies {
		websiteData.Technologies = detectTechnologies(websiteData)
	}

	return websiteData, parseErr
}

// parseWithColly uses the Colly crawler for basic scraping
func parseWithColly(ctx context.Context, websiteData *WebsiteData, parsedURL *url.URL, opts ParseOptions) error {
	// Set up the collector
	c := colly.NewCollector(
		colly.AllowedDomains(parsedURL.Hostname()),
		colly.MaxDepth(opts.MaxDepth),
		colly.Async(true),
	)

	// Set rate limiting with adaptive configuration for concurrency
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: opts.Concurrency,
		Delay:       1 * time.Second,
		RandomDelay: 500 * time.Millisecond,
	})

	// Set user agent
	if opts.UserAgent != "" {
		c.UserAgent = opts.UserAgent
	} else {
		extensions.RandomUserAgent(c)
	}

	if opts.RespectRobotsTxt {
		extensions.Referer(c)
	}

	// Set proxy if specified
	if opts.ProxyURL != "" {
		c.SetProxy(opts.ProxyURL)
	}

	// Add custom headers
	if len(opts.Headers) > 0 {
		c.OnRequest(func(r *colly.Request) {
			for key, value := range opts.Headers {
				r.Headers.Set(key, value)
			}
		})
	}

	// Add cookies
	if len(opts.Cookies) > 0 {
		c.OnRequest(func(r *colly.Request) {
			for _, cookie := range opts.Cookies {
				r.Headers.Add("Cookie", cookie.Name+"="+cookie.Value)
			}
		})
	}

	c.SetRequestTimeout(opts.Timeout)

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
	})

	// Advanced retry logic with exponential backoff
	var lastErr error
	retryCount := 0

	for retryCount <= opts.MaxRetries {
		// Only retry if previous attempt failed
		if retryCount > 0 && lastErr == nil {
			break
		}

		c.OnError(func(r *colly.Response, err error) {
			websiteData.StatusCode = r.StatusCode
			if r.StatusCode == 0 {
				websiteData.StatusCode = http.StatusInternalServerError
			}
			lastErr = err
		})

		// Start scraping
		err := c.Visit(websiteData.URL)
		if err != nil {
			lastErr = err
			retryCount++

			if retryCount <= opts.MaxRetries {
				// Exponential backoff with jitter
				baseDelay := opts.RetryDelay * time.Duration(1<<uint(retryCount-1))
				jitter := time.Duration(float64(baseDelay) * 0.2 * (0.5 + rand.Float64()))
				delay := baseDelay + jitter

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
			// Return the last error if all retries failed
			return fmt.Errorf("failed after %d retries: %w", opts.MaxRetries, err)
		}

		// Wait for all goroutines to finish
		c.Wait()
		break
	}

	// Check link statuses after initial parsing with optimized parallel execution
	if len(websiteData.Links) > 0 {
		if err := checkLinksStatus(ctx, websiteData, opts); err != nil {
			return fmt.Errorf("error checking links: %w", err)
		}
	}

	// Estimate image file sizes
	if err := estimateImageSizes(ctx, websiteData, opts); err != nil {
		return fmt.Errorf("error estimating image sizes: %w", err)
	}

	return nil
}

// parseWithHeadlessBrowser uses Chrome/Chromium through chromedp for JavaScript-heavy sites
func parseWithHeadlessBrowser(ctx context.Context, websiteData *WebsiteData, targetURL string, opts ParseOptions) error {
	// Create timeout context
	jsCtx, jsCancel := context.WithTimeout(ctx, opts.JavaScriptTimeout)
	defer jsCancel()

	// Set up browser options
	chromeOpts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.WindowSize(1920, 1080),
	}

	if opts.BypassAntiBot {
		// Add options to bypass anti-bot measures
		chromeOpts = append(chromeOpts,
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-web-security", true),
			chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
			chromedp.Flag("disable-site-isolation-trials", true),
		)
	}

	// Set custom path for Chrome if specified
	if opts.CustomChromePath != "" {
		chromeOpts = append(chromeOpts, chromedp.ExecPath(opts.CustomChromePath))
	}

	// Set proxy if specified
	if opts.ProxyURL != "" {
		chromeOpts = append(chromeOpts, chromedp.ProxyServer(opts.ProxyURL))
	}

	// Add user agent
	if opts.UserAgent != "" {
		chromeOpts = append(chromeOpts, chromedp.UserAgent(opts.UserAgent))
	}

	// Create browser context
	allocCtx, allocCancel := chromedp.NewExecAllocator(jsCtx, chromeOpts...)
	defer allocCancel()

	// Create browser tab context with logging
	taskCtx, taskCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}))
	defer taskCancel()

	// Determine which devices to use for screenshots
	screenshotDevices := opts.ScreenshotDevices
	if opts.CaptureScreenshots && len(screenshotDevices) == 0 {
		// Default to desktop and mobile if not specified
		screenshotDevices = []DeviceConfig{DesktopDevice, MobileDevice}
	}

	// Extract main page content first
	var html, title, pageText string
	var extractedData struct {
		Headings struct {
			H1 []string `json:"h1"`
			H2 []string `json:"h2"`
			H3 []string `json:"h3"`
		} `json:"headings"`
		MetaTags map[string]string `json:"metaTags"`
		Links    []struct {
			URL      string `json:"url"`
			Text     string `json:"text"`
			NoFollow bool   `json:"noFollow"`
		} `json:"links"`
		Images []struct {
			URL    string `json:"url"`
			Alt    string `json:"alt"`
			Width  string `json:"width"`
			Height string `json:"height"`
		} `json:"images"`
		Scripts []struct {
			URL        string `json:"url"`
			Type       string `json:"type"`
			IsAsync    bool   `json:"isAsync"`
			IsDeferred bool   `json:"isDeferred"`
		} `json:"scripts"`
		Styles []struct {
			URL    string `json:"url"`
			Media  string `json:"media"`
			IsLink bool   `json:"isLink"`
		} `json:"styles"`
	}

	// Basic actions for the default device
	tasks := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(targetURL),
	}

	// Add wait actions
	if opts.WaitForSelector != "" {
		tasks = append(tasks,
			chromedp.WaitVisible(opts.WaitForSelector, chromedp.ByQuery),
		)
	}

	if opts.WaitTime > 0 {
		tasks = append(tasks, chromedp.Sleep(opts.WaitTime))
	}

	// Add cookies if specified
	if len(opts.Cookies) > 0 {
		for _, cookie := range opts.Cookies {
			tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
				return network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					Do(ctx)
			}))

		}
	}

	// Add headers if specified
	if len(opts.Headers) > 0 {
		headerParams := network.Headers{}
		for key, value := range opts.Headers {
			headerParams[key] = value
		}

		tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetExtraHTTPHeaders(headerParams).Do(ctx)
		}))
	}

	// Add extract tasks
	tasks = append(tasks,
		chromedp.OuterHTML("html", &html),
		chromedp.Title(&title),
		chromedp.Text("body", &pageText),
		// Extract page data using JavaScript
		chromedp.Evaluate(`
			(() => {
				const result = {
					headings: { h1: [], h2: [], h3: [] },
					metaTags: {},
					links: [],
					images: [],
					scripts: [],
					styles: []
				};

				// Collect headings
				document.querySelectorAll('h1').forEach(el => 
					result.headings.h1.push(el.textContent.trim())
				);
				document.querySelectorAll('h2').forEach(el => 
					result.headings.h2.push(el.textContent.trim())
				);
				document.querySelectorAll('h3').forEach(el => 
					result.headings.h3.push(el.textContent.trim())
				);

				// Collect meta tags
				document.querySelectorAll('meta').forEach(el => {
					const name = el.getAttribute('name') || el.getAttribute('property');
					const content = el.getAttribute('content');
					if (name && content) {
						result.metaTags[name] = content;
					}
				});

				// Collect links
				document.querySelectorAll('a[href]').forEach(el => {
					const href = el.getAttribute('href');
					if (href && href !== '#' && !href.startsWith('javascript:')) {
						result.links.push({
							url: el.href, // Use href property for absolute URL
							text: el.textContent.trim(),
							noFollow: el.getAttribute('rel') === 'nofollow'
						});
					}
				});

				// Collect images
				document.querySelectorAll('img[src]').forEach(el => {
					const src = el.getAttribute('src');
					if (src) {
						result.images.push({
							url: el.src, // Use src property for absolute URL
							alt: el.getAttribute('alt') || '',
							width: el.getAttribute('width') || '',
							height: el.getAttribute('height') || ''
						});
					}
				});

				// Collect scripts
				document.querySelectorAll('script[src]').forEach(el => {
					result.scripts.push({
						url: el.src, // Use src property for absolute URL
						type: el.getAttribute('type') || '',
						isAsync: el.hasAttribute('async'),
						isDeferred: el.hasAttribute('defer')
					});
				});

				// Collect styles
				document.querySelectorAll('link[rel="stylesheet"]').forEach(el => {
					result.styles.push({
						url: el.href, // Use href property for absolute URL
						media: el.getAttribute('media') || '',
						isLink: true
					});
				});
				document.querySelectorAll('style').forEach(el => {
					result.styles.push({
						url: '',
						media: el.getAttribute('media') || '',
						isLink: false
					});
				});

				return result;
			})()
		`, &extractedData),
	)

	// Execute the tasks
	if err := chromedp.Run(taskCtx, tasks...); err != nil {
		return fmt.Errorf("failed to execute browser tasks: %w", err)
	}

	// Process parsed data
	websiteData.HTML = html
	websiteData.Title = title
	websiteData.TextContent = pageText
	websiteData.H1 = extractedData.Headings.H1
	websiteData.H2 = extractedData.Headings.H2
	websiteData.H3 = extractedData.Headings.H3
	websiteData.MetaTags = extractedData.MetaTags

	// Process description and keywords from meta tags
	if desc, ok := extractedData.MetaTags["description"]; ok {
		websiteData.Description = desc
	}
	if keywords, ok := extractedData.MetaTags["keywords"]; ok {
		websiteData.Keywords = keywords
	}

	// Process links
	processedURLs := make(map[string]bool)
	for _, link := range extractedData.Links {
		if _, processed := processedURLs[link.URL]; processed {
			continue // Skip duplicates
		}

		processedURLs[link.URL] = true

		// Determine if link is internal
		linkURL, err := url.Parse(link.URL)
		parsedTargetURL, _ := url.Parse(targetURL)
		isInternal := err == nil && linkURL.Hostname() == parsedTargetURL.Hostname()

		websiteData.Links = append(websiteData.Links, Link{
			URL:        link.URL,
			Text:       link.Text,
			IsInternal: isInternal,
			NoFollow:   link.NoFollow,
		})
	}

	// Process images
	for _, img := range extractedData.Images {
		if strings.TrimSpace(img.URL) == "" {
			continue
		}

		websiteData.Images = append(websiteData.Images, Image{
			URL:    img.URL,
			Alt:    img.Alt,
			Width:  img.Width,
			Height: img.Height,
		})
	}

	// Process scripts
	for _, script := range extractedData.Scripts {
		if strings.TrimSpace(script.URL) == "" {
			continue
		}

		websiteData.Scripts = append(websiteData.Scripts, Script{
			URL:        script.URL,
			Type:       script.Type,
			IsAsync:    script.IsAsync,
			IsDeferred: script.IsDeferred,
		})
	}

	// Process styles
	for _, style := range extractedData.Styles {
		websiteData.Styles = append(websiteData.Styles, Style{
			URL:    style.URL,
			Media:  style.Media,
			IsLink: style.IsLink,
		})
	}

	// Take screenshots if requested
	if opts.CaptureScreenshots && len(screenshotDevices) > 0 {
		for _, device := range screenshotDevices {
			deviceCtx, deviceCancel := chromedp.NewContext(allocCtx)
			defer deviceCancel() // не забываем освобождать ресурсы

			// Создаем сначала tasks с эмуляцией устройства
			deviceTasks := []chromedp.Action{
				// Эмулируем метрики устройства
				emulation.SetDeviceMetricsOverride(
					int64(device.Width),
					int64(device.Height),
					device.DeviceScaleFactor,
					device.Mobile),
			}

			// Добавляем user agent через правильный API
			if device.UserAgent != "" {
				deviceTasks = append(deviceTasks,
					chromedp.ActionFunc(func(ctx context.Context) error {
						return emulation.SetUserAgentOverride(device.UserAgent).Do(ctx)
					}),
				)
			}

			// Навигация и ожидание
			deviceTasks = append(deviceTasks,
				chromedp.Navigate(targetURL),
			)

			if opts.WaitForSelector != "" {
				deviceTasks = append(deviceTasks,
					chromedp.WaitVisible(opts.WaitForSelector),
				)
			}

			if opts.WaitTime > 0 {
				deviceTasks = append(deviceTasks,
					chromedp.Sleep(opts.WaitTime),
				)
			}

			// Захват скриншота
			var screenshot []byte
			deviceTasks = append(deviceTasks,
				chromedp.ActionFunc(func(ctx context.Context) error {
					return chromedp.FullScreenshot(&screenshot, 90).Do(ctx)
				}),
			)

			// Выполняем все задачи
			if err := chromedp.Run(deviceCtx, deviceTasks...); err != nil {
				// Логируем ошибку, но продолжаем работу
				fmt.Printf("Failed to capture screenshot for %s: %v\n", device.Name, err)
				continue
			}

			// Сохраняем скриншот
			websiteData.Screenshots[device.Name] = screenshot
		}
	}

	// Check link statuses after parsing
	if len(websiteData.Links) > 0 {
		if err := checkLinksStatus(ctx, websiteData, opts); err != nil {
			return fmt.Errorf("error checking links: %w", err)
		}
	}

	// Estimate image file sizes
	if err := estimateImageSizes(ctx, websiteData, opts); err != nil {
		return fmt.Errorf("error estimating image sizes: %w", err)
	}

	return nil
}

// checkLinksStatus checks the HTTP status of links with improved error handling and parallel execution
func checkLinksStatus(ctx context.Context, data *WebsiteData, opts ParseOptions) error {
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, opts.Concurrency)

	// Use a client with connection pooling and timeout
	client := &http.Client{
		Timeout: opts.Timeout / 2, // Shorter timeout for individual links
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Use a mutex to protect concurrent modifications to the links array
	var mu sync.Mutex

	// Track errors to avoid failing on individual link failures
	var errs []error
	var errMu sync.Mutex

	for i, link := range data.Links {
		// Skip external links unless specifically requested
		if !link.IsInternal && !opts.CheckExternalURLs {
			continue
		}

		// Add to wait group
		wg.Add(1)

		// Acquire semaphore
		semaphore <- struct{}{}

		go func(i int, link Link) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done

			statusCode := 0

			// Implement retry logic
			for retryCount := 0; retryCount <= opts.MaxRetries; retryCount++ {
				select {
				case <-ctx.Done():
					// Context cancelled/timed out
					statusCode = http.StatusRequestTimeout
					return
				default:
					// Continue with request
				}

				// Create request
				req, err := http.NewRequestWithContext(ctx, "HEAD", link.URL, nil)
				if err != nil {
					if retryCount == opts.MaxRetries {
						errMu.Lock()
						errs = append(errs, fmt.Errorf("error creating request for %s: %w", link.URL, err))
						errMu.Unlock()
					}
					continue
				}

				// Set headers
				req.Header.Set("User-Agent", opts.UserAgent)
				for key, value := range opts.Headers {
					req.Header.Set(key, value)
				}

				// Make request
				resp, err := client.Do(req)
				if err != nil {
					// If this is the last retry, log the error
					if retryCount == opts.MaxRetries {
						statusCode = http.StatusInternalServerError
						errMu.Lock()
						errs = append(errs, fmt.Errorf("error checking %s after %d retries: %w", link.URL, opts.MaxRetries, err))
						errMu.Unlock()
					}

					// Wait before retrying with exponential backoff and jitter
					if retryCount < opts.MaxRetries {
						baseDelay := opts.RetryDelay * time.Duration(1<<uint(retryCount))
						jitter := time.Duration(float64(baseDelay) * 0.2 * (0.5 + rand.Float64()))
						delay := baseDelay + jitter

						select {
						case <-ctx.Done():
							return
						case <-time.After(delay):
							continue
						}
					}
				} else {
					statusCode = resp.StatusCode
					resp.Body.Close()
					break
				}
			}

			// Update status code with mutex protection
			mu.Lock()
			data.Links[i].StatusCode = statusCode
			mu.Unlock()
		}(i, link)
	}

	// Wait for all link checks to complete
	wg.Wait()

	// If there were multiple errors, combine them
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors while checking links", len(errs))
	}

	return nil
}

// estimateImageSizes tries to get the file size of images with improved error handling
func estimateImageSizes(ctx context.Context, data *WebsiteData, opts ParseOptions) error {
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, opts.Concurrency)

	// Use a client with connection pooling and timeout
	client := &http.Client{
		Timeout: opts.Timeout / 2,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Use a mutex to protect concurrent modifications to the images array
	var mu sync.Mutex

	for i, img := range data.Images {
		// Skip data URLs and empty URLs
		if strings.HasPrefix(img.URL, "data:") || strings.TrimSpace(img.URL) == "" {
			continue
		}

		// Add to wait group
		wg.Add(1)

		// Acquire semaphore
		semaphore <- struct{}{}

		go func(i int, img Image) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done

			// Implement retry logic
			for retryCount := 0; retryCount <= opts.MaxRetries; retryCount++ {
				select {
				case <-ctx.Done():
					// Context cancelled/timed out
					return
				default:
					// Continue with request
				}

				// Create request
				req, err := http.NewRequestWithContext(ctx, "HEAD", img.URL, nil)
				if err != nil {
					continue
				}

				// Set headers
				req.Header.Set("User-Agent", opts.UserAgent)
				for key, value := range opts.Headers {
					req.Header.Set(key, value)
				}

				// Make request
				resp, err := client.Do(req)
				if err != nil {
					// Wait before retrying with exponential backoff and jitter
					if retryCount < opts.MaxRetries {
						baseDelay := opts.RetryDelay * time.Duration(1<<uint(retryCount))
						jitter := time.Duration(float64(baseDelay) * 0.2 * (0.5 + rand.Float64()))
						delay := baseDelay + jitter

						select {
						case <-ctx.Done():
							return
						case <-time.After(delay):
							continue
						}
					}
				} else {
					// Success - update file size with mutex protection
					if resp.StatusCode == http.StatusOK {
						mu.Lock()
						data.Images[i].FileSize = resp.ContentLength
						mu.Unlock()
					}
					resp.Body.Close()
					break
				}
			}
		}(i, img)
	}

	// Wait for all image size checks to complete
	wg.Wait()

	return nil
}

// Technology signature for detection
type techSignature struct {
	Name         string
	Category     string
	PatternType  string // "js", "html", "meta", "header", "cookie"
	Pattern      string
	Version      string
	VersionRegex string
	Confidence   int
}

// detectTechnologies identifies technologies used on the website
func detectTechnologies(data *WebsiteData) []Technology {
	technologies := []Technology{}
	techMap := make(map[string]Technology) // Used to deduplicate findings

	// Load technology signatures
	signatures := getTechnologySignatures()

	// Check JavaScript libraries signatures
	for _, signature := range signatures {
		switch signature.PatternType {
		case "js":
			// Check scripts
			for _, script := range data.Scripts {
				if strings.Contains(script.URL, signature.Pattern) {
					tech := Technology{
						Name:       signature.Name,
						Category:   signature.Category,
						Confidence: signature.Confidence,
					}

					// Try to extract version if version regex is defined
					if signature.VersionRegex != "" && script.URL != "" {
						versionRE := regexp.MustCompile(signature.VersionRegex)
						matches := versionRE.FindStringSubmatch(script.URL)
						if len(matches) > 1 {
							tech.Version = matches[1]
						}
					}

					techMap[signature.Name] = tech
				}
			}

		case "html":
			// Check HTML for patterns
			if strings.Contains(data.HTML, signature.Pattern) {
				tech := Technology{
					Name:       signature.Name,
					Category:   signature.Category,
					Confidence: signature.Confidence,
				}

				// Try to extract version if version regex is defined
				if signature.VersionRegex != "" {
					versionRE := regexp.MustCompile(signature.VersionRegex)
					matches := versionRE.FindStringSubmatch(data.HTML)
					if len(matches) > 1 {
						tech.Version = matches[1]
					}
				}

				techMap[signature.Name] = tech
			}

		case "meta":
			// Check meta tags
			for name, content := range data.MetaTags {
				if name == signature.Pattern || content == signature.Pattern ||
					(signature.Pattern != "" && strings.Contains(content, signature.Pattern)) {
					tech := Technology{
						Name:       signature.Name,
						Category:   signature.Category,
						Confidence: signature.Confidence,
					}

					// Try to extract version if version regex is defined
					if signature.VersionRegex != "" {
						versionRE := regexp.MustCompile(signature.VersionRegex)
						matches := versionRE.FindStringSubmatch(content)
						if len(matches) > 1 {
							tech.Version = matches[1]
						}
					}

					techMap[signature.Name] = tech
				}
			}
		}
	}

	// Check for common CMS systems
	cmsDetected := detectCMS(data)
	for name, tech := range cmsDetected {
		techMap[name] = tech
	}

	// Convert map to slice
	for _, tech := range techMap {
		technologies = append(technologies, tech)
	}

	return technologies
}

// detectCMS identifies Content Management Systems
func detectCMS(data *WebsiteData) map[string]Technology {
	cms := make(map[string]Technology)

	// WordPress detection
	if strings.Contains(data.HTML, "wp-content") ||
		strings.Contains(data.HTML, "wp-includes") {
		cms["WordPress"] = Technology{
			Name:        "WordPress",
			Category:    "CMS",
			Confidence:  90,
			Description: "WordPress is a free and open-source content management system",
			Website:     "https://wordpress.org",
		}

		// Try to detect version
		versionRE := regexp.MustCompile(`meta name="generator" content="WordPress ([0-9.]+)"`)
		matches := versionRE.FindStringSubmatch(data.HTML)
		if len(matches) > 1 {
			tech := cms["WordPress"]
			tech.Version = matches[1]
			cms["WordPress"] = tech
		}
	}

	// Drupal detection
	if strings.Contains(data.HTML, "Drupal.settings") ||
		strings.Contains(data.HTML, "drupal.org") {
		cms["Drupal"] = Technology{
			Name:        "Drupal",
			Category:    "CMS",
			Confidence:  90,
			Description: "Drupal is a free and open-source content management framework",
			Website:     "https://www.drupal.org",
		}
	}

	// Joomla detection
	if strings.Contains(data.HTML, "/media/system/js/core.js") ||
		strings.Contains(data.HTML, "/media/jui/") {
		cms["Joomla"] = Technology{
			Name:        "Joomla",
			Category:    "CMS",
			Confidence:  90,
			Description: "Joomla is a free and open-source content management system",
			Website:     "https://www.joomla.org",
		}
	}

	// Check for frameworks
	for _, script := range data.Scripts {
		// React
		if strings.Contains(script.URL, "react.") ||
			strings.Contains(script.URL, "react-dom") {
			cms["React"] = Technology{
				Name:        "React",
				Category:    "JavaScript Framework",
				Confidence:  90,
				Description: "React is a JavaScript library for building user interfaces",
				Website:     "https://reactjs.org",
			}
		}

		// Vue.js
		if strings.Contains(script.URL, "vue.") ||
			strings.Contains(data.HTML, "data-v-") {
			cms["Vue.js"] = Technology{
				Name:        "Vue.js",
				Category:    "JavaScript Framework",
				Confidence:  90,
				Description: "Vue.js is a progressive JavaScript framework for building user interfaces",
				Website:     "https://vuejs.org",
			}
		}

		// Angular
		if strings.Contains(script.URL, "angular.") ||
			strings.Contains(data.HTML, "ng-app") ||
			strings.Contains(data.HTML, "ng-controller") {
			cms["Angular"] = Technology{
				Name:        "Angular",
				Category:    "JavaScript Framework",
				Confidence:  90,
				Description: "Angular is a TypeScript-based open-source web application framework",
				Website:     "https://angular.io",
			}
		}
	}

	return cms
}

// Helper function to get technology signatures (simplified version)
func getTechnologySignatures() []techSignature {
	return []techSignature{
		{
			Name:         "jQuery",
			Category:     "JavaScript Library",
			PatternType:  "js",
			Pattern:      "jquery",
			VersionRegex: `jquery[.-]([0-9.]+)`,
			Confidence:   90,
		},
		{
			Name:         "Bootstrap",
			Category:     "UI Framework",
			PatternType:  "js",
			Pattern:      "bootstrap",
			VersionRegex: `bootstrap[.-]([0-9.]+)`,
			Confidence:   90,
		},
		{
			Name:        "Google Analytics",
			Category:    "Analytics",
			PatternType: "js",
			Pattern:     "google-analytics.com/analytics.js",
			Confidence:  100,
		},
		{
			Name:        "Google Tag Manager",
			Category:    "Tag Manager",
			PatternType: "js",
			Pattern:     "googletagmanager.com",
			Confidence:  100,
		},
		{
			Name:        "Cloudflare",
			Category:    "CDN",
			PatternType: "html",
			Pattern:     "cloudflare",
			Confidence:  80,
		},
		{
			Name:        "Shopify",
			Category:    "E-commerce",
			PatternType: "html",
			Pattern:     "cdn.shopify.com",
			Confidence:  100,
		},
		{
			Name:        "WooCommerce",
			Category:    "E-commerce",
			PatternType: "html",
			Pattern:     "woocommerce",
			Confidence:  90,
		},
		{
			Name:        "Magento",
			Category:    "E-commerce",
			PatternType: "html",
			Pattern:     "Magento",
			Confidence:  80,
		},
		{
			Name:        "Next.js",
			Category:    "React Framework",
			PatternType: "html",
			Pattern:     "next/",
			Confidence:  80,
		},
		{
			Name:         "Gatsby",
			Category:     "React Framework",
			PatternType:  "meta",
			Pattern:      "generator",
			VersionRegex: `Gatsby ([0-9.]+)`,
			Confidence:   100,
		},
		{
			Name:         "WordPress",
			Category:     "CMS",
			PatternType:  "meta",
			Pattern:      "generator",
			VersionRegex: `WordPress ([0-9.]+)`,
			Confidence:   100,
		},
		{
			Name:        "Laravel",
			Category:    "PHP Framework",
			PatternType: "html",
			Pattern:     "laravel",
			Confidence:  80,
		},
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
