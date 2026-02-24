// Package db provides TLDR Pages API client for WUT
package db

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	baseRawURL = "https://raw.githubusercontent.com/tldr-pages/tldr/main"
	// Platforms available in tldr-pages
	PlatformCommon  = "common"
	PlatformLinux   = "linux"
	PlatformMacOS   = "osx"
	PlatformWindows = "windows"
	PlatformSunOS   = "sunos"
	PlatformAndroid = "android"
	PlatformFreeBSD = "freebsd"
	PlatformNetBSD  = "netbsd"
	PlatformOpenBSD = "openbsd"
)

// Client represents the TLDR API client
type Client struct {
	httpClient    *http.Client
	baseURL       string
	language      string
	storage       *Storage
	offlineMode   bool
	autoDetect    bool
	cacheInMemory bool
	memoryCache   map[string]*Page
	cacheMu       sync.RWMutex
}

// Page represents a TLDR page with parsed content
type Page struct {
	Name        string
	Platform    string
	Language    string
	Description string
	Examples    []Example
	RawContent  string
}

// Example represents a command example from TLDR
type Example struct {
	Description string
	Command     string
}

// ClientOption is a functional option for Client
type ClientOption func(*Client)

// WithStorage sets the local storage for offline support
func WithStorage(storage *Storage) ClientOption {
	return func(c *Client) {
		c.storage = storage
	}
}

// WithOfflineMode enables offline-only mode
func WithOfflineMode(offline bool) ClientOption {
	return func(c *Client) {
		c.offlineMode = offline
	}
}

// WithAutoDetect enables auto-detection of online/offline mode
func WithAutoDetect(auto bool) ClientOption {
	return func(c *Client) {
		c.autoDetect = auto
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithLanguage sets the preferred language
func WithLanguage(lang string) ClientOption {
	return func(c *Client) {
		c.language = lang
	}
}

// NewClient creates a new TLDR API client
func NewClient(opts ...ClientOption) *Client {
	lang := "en"

	c := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:       baseRawURL,
		language:      lang,
		offlineMode:   false,
		autoDetect:    true,
		cacheInMemory: true,
		memoryCache:   make(map[string]*Page),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetHTTPClient sets a custom HTTP client (useful for testing)
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// SetStorage sets the local storage
func (c *Client) SetStorage(storage *Storage) {
	c.storage = storage
}

// SetOfflineMode enables or disables offline-only mode
func (c *Client) SetOfflineMode(offline bool) {
	c.offlineMode = offline
}

// SetAutoDetect enables or disables auto-detection
func (c *Client) SetAutoDetect(auto bool) {
	c.autoDetect = auto
}

// IsOfflineMode returns true if client is in offline mode
func (c *Client) IsOfflineMode() bool {
	return c.offlineMode
}

// IsOnline checks if the client can connect to the internet
func (c *Client) IsOnline(ctx context.Context) bool {
	if c.offlineMode {
		return false
	}

	// Try to fetch a small page to check connectivity
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/pages/%s/%s.md", c.baseURL, PlatformCommon, "ls")
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetPage retrieves a TLDR page for a specific command and platform
// Auto-detects online/offline and falls back to local storage automatically
func (c *Client) GetPage(ctx context.Context, command, platform string) (*Page, error) {
	lang := c.language
	if lang == "" {
		lang = "en"
	}
	cacheKey := fmt.Sprintf("%s/%s/%s", lang, platform, command)

	// Check memory cache first
	if c.cacheInMemory {
		c.cacheMu.RLock()
		if page, ok := c.memoryCache[cacheKey]; ok {
			c.cacheMu.RUnlock()
			return page, nil
		}
		c.cacheMu.RUnlock()
	}

	// Check local storage second
	if c.storage != nil {
		page, err := c.storage.GetPage(command, platform, lang)
		if err == nil {
			// Cache in memory
			if c.cacheInMemory {
				c.cacheMu.Lock()
				c.memoryCache[cacheKey] = page
				c.cacheMu.Unlock()
			}
			return page, nil
		}
	}

	// If offline mode, don't try remote
	if c.offlineMode {
		return nil, fmt.Errorf("page not found in local storage (offline mode): %s/%s", platform, command)
	}

	// Try to fetch from remote
	var langDir string
	if lang == "en" {
		langDir = "pages"
	} else {
		langDir = "pages." + lang
	}
	url := fmt.Sprintf("%s/%s/%s/%s.md", c.baseURL, langDir, platform, command)
	content, err := c.fetch(ctx, url)

	if err != nil && lang != "en" {
		// Fallback to english if not found
		fallbackURL := fmt.Sprintf("%s/pages/%s/%s.md", c.baseURL, platform, command)
		content, err = c.fetch(ctx, fallbackURL)
		if err == nil {
			lang = "en"
		}
	}

	if err != nil {
		// Network error - auto fall back to offline mode if autoDetect is enabled
		if c.autoDetect {
			c.offlineMode = true
			return nil, fmt.Errorf("offline mode: page not found in local storage: %s/%s (use 'wut db sync' to download)", platform, command)
		}
		return nil, err
	}

	// Parse and save
	page := c.parsePage(content, command, platform, lang)

	// Save to local storage if available
	if c.storage != nil {
		_ = c.storage.SavePage(page)
	}

	// Cache in memory
	if c.cacheInMemory {
		c.cacheMu.Lock()
		c.memoryCache[cacheKey] = page
		c.cacheMu.Unlock()
	}

	return page, nil
}

// SearchPages searches for TLDR pages across all platforms
func (c *Client) SearchPages(ctx context.Context, query string) ([]Page, error) {
	// Try local storage first if offline mode or auto-detect
	if c.offlineMode || (c.autoDetect && !c.IsOnline(ctx)) {
		if c.storage != nil {
			storedPages, err := c.storage.SearchLocal(query)
			if err == nil && len(storedPages) > 0 {
				pages := make([]Page, len(storedPages))
				for i, sp := range storedPages {
					pages[i] = Page{
						Name:        sp.Name,
						Platform:    sp.Platform,
						Description: sp.Description,
						Examples:    sp.Examples,
						RawContent:  sp.RawContent,
					}
				}
				return pages, nil
			}
		}
	}

	platforms := []string{
		PlatformCommon,
		PlatformLinux,
		PlatformMacOS,
		PlatformWindows,
	}

	var pages []Page
	seen := make(map[string]bool)

	for _, platform := range platforms {
		page, err := c.GetPage(ctx, query, platform)
		if err != nil {
			continue
		}

		// Avoid duplicates
		key := page.Name + page.Description
		if !seen[key] {
			seen[key] = true
			pages = append(pages, *page)
		}
	}

	return pages, nil
}

// GetPageAnyPlatform tries to get a page from any available platform
// Auto-detects online/offline and falls back automatically
func (c *Client) GetPageAnyPlatform(ctx context.Context, command string) (*Page, error) {
	// Check memory cache first
	if c.cacheInMemory {
		c.cacheMu.RLock()
		for _, page := range c.memoryCache {
			if page.Name == command {
				c.cacheMu.RUnlock()
				return page, nil
			}
		}
		c.cacheMu.RUnlock()
	}

	// Check local storage second
	lang := c.language
	if lang == "" {
		lang = "en"
	}
	if c.storage != nil {
		page, err := c.storage.GetPageAnyPlatform(command, lang)
		if err == nil {
			// Cache in memory
			if c.cacheInMemory {
				c.cacheMu.Lock()
				c.memoryCache[fmt.Sprintf("%s/%s/%s", page.Language, page.Platform, page.Name)] = page
				c.cacheMu.Unlock()
			}
			return page, nil
		}
	}

	// If offline mode, don't try remote
	if c.offlineMode {
		return nil, fmt.Errorf("page not found in local storage (offline mode): %s", command)
	}

	// Try to fetch from remote with auto fallback
	platforms := []string{
		PlatformCommon,
		PlatformLinux,
		PlatformMacOS,
		PlatformWindows,
		PlatformFreeBSD,
		PlatformOpenBSD,
		PlatformNetBSD,
		PlatformSunOS,
		PlatformAndroid,
	}

	for _, platform := range platforms {
		page, err := c.GetPage(ctx, command, platform)
		if err == nil {
			return page, nil
		}
	}

	// If all failed and autoDetect is enabled, switch to offline mode
	if c.autoDetect {
		c.offlineMode = true
	}

	return nil, fmt.Errorf("page not found for command: %s", command)
}

// fetch retrieves raw content from the given URL
func (c *Client) fetch(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("page not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	return string(body), nil
}

// parsePage parses raw markdown content into a Page struct
func (c *Client) parsePage(content, name, platform, language string) *Page {
	if language == "" {
		language = "en"
	}
	page := &Page{
		Name:       name,
		Platform:   platform,
		Language:   language,
		RawContent: content,
		Examples:   []Example{},
	}

	lines := strings.Split(content, "\n")
	var inExample bool
	var currentExample Example

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// Title line (starts with #)
		if after, ok := strings.CutPrefix(line, "# "); ok {
			page.Name = after
			continue
		}

		// Description line (starts with >)
		if after, ok := strings.CutPrefix(line, "> "); ok {
			page.Description = after
			continue
		}

		// Example description (starts with -)
		if strings.HasPrefix(line, "- ") {
			// Save previous example if exists
			if currentExample.Command != "" {
				page.Examples = append(page.Examples, currentExample)
			}
			currentExample = Example{
				Description: strings.TrimPrefix(line, "- "),
			}
			inExample = true
			continue
		}

		// Example command (starts with `)
		if inExample && strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") {
			cmd := strings.Trim(line, "`")
			// Replace {{variable}} with <variable>
			cmd = formatCommand(cmd)
			currentExample.Command = cmd
			inExample = false

			// Save the example
			if currentExample.Description != "" {
				page.Examples = append(page.Examples, currentExample)
				currentExample = Example{}
			}
		}
	}

	return page
}

// formatCommand formats a command by replacing {{variable}} placeholders
func formatCommand(cmd string) string {
	// Replace {{variable}} with <variable>
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllString(cmd, "<$1>")
}

// GetAvailableCommands returns a list of available commands from local storage
// or a default list if local storage is empty
func (c *Client) GetAvailableCommands(ctx context.Context) ([]string, error) {
	// Try local storage first
	if c.storage != nil {
		pages, err := c.storage.GetAllPages()
		if err == nil && len(pages) > 0 {
			commands := make([]string, len(pages))
			for i, page := range pages {
				commands[i] = page.Name
			}
			return commands, nil
		}
	}

	// Return default list
	return getDefaultCommands(), nil
}

// getDefaultCommands returns the default list of common commands
func getDefaultCommands() []string {
	return []string{
		"git", "docker", "npm", "node", "python", "pip", "cargo",
		"kubectl", "helm", "terraform", "ansible", "vagrant",
		"ls", "cd", "pwd", "cat", "less", "head", "tail",
		"grep", "find", "sed", "awk", "sort", "wc",
		"tar", "zip", "unzip", "gzip",
		"chmod", "chown", "mkdir", "rm", "cp", "mv",
		"ps", "htop", "kill", "killall",
		"ssh", "scp", "rsync", "curl", "wget", "ping", "netstat",
		"vim", "vi", "nano",
		"make", "cmake", "gcc", "clang",
		"ffmpeg",
	}
}

// HasLocalStorage returns true if client has local storage configured
func (c *Client) HasLocalStorage() bool {
	return c.storage != nil
}

// GetStorage returns the local storage
func (c *Client) GetStorage() *Storage {
	return c.storage
}

// ClearMemoryCache clears the in-memory cache
func (c *Client) ClearMemoryCache() {
	c.cacheMu.Lock()
	c.memoryCache = make(map[string]*Page)
	c.cacheMu.Unlock()
}

// GetMemoryCacheSize returns the number of pages in memory cache
func (c *Client) GetMemoryCacheSize() int {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return len(c.memoryCache)
}
