package walmart

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/eshaffer321/walmart-client-go/v2/internal/cookies"
)

var getOrderHashPattern = regexp.MustCompile(`/orchestra/orders/graphql/getOrder/([a-f0-9]{64})([/?]|$)`)

// WalmartClient provides access to Walmart's order history and purchase data API.
type WalmartClient struct {
	httpClient        *http.Client
	cookieStore       *cookies.Store
	rateLimiter       *time.Ticker
	ledgerRateLimiter *time.Ticker
	lastRequest       time.Time
	lastLedgerRequest time.Time
	maxRetries        int
	mu                sync.RWMutex
	logger            *slog.Logger
}

// NewWalmartClient creates a new Walmart API client with the given configuration.
func NewWalmartClient(config ClientConfig) (*WalmartClient, error) {
	// Set defaults.
	if config.CookieFile == "" {
		if config.CookieDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to determine home directory: %w", err)
			}
			config.CookieDir = filepath.Join(homeDir, ".walmart-api")
		}
		if err := os.MkdirAll(config.CookieDir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create cookie directory: %w", err)
		}
		config.CookieFile = filepath.Join(config.CookieDir, "cookies.json")
	}

	if config.RateLimit == 0 {
		config.RateLimit = 2 * time.Second
	}

	// Set ledger rate limit (defaults to regular rate limit if not specified).
	ledgerRate := config.LedgerRateLimit
	if ledgerRate == 0 {
		ledgerRate = config.RateLimit
	}

	// Set max retries (defaults to 3 if not specified).
	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	// Use provided logger or no-op logger if none provided
	// Note: Caller is responsible for adding any scoping attributes (e.g., system="walmart").
	logger := config.Logger
	if logger == nil {
		// Use no-op logger that discards all output.
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Initialize cookie store.
	store := cookies.NewStore(config.CookieFile, logger)

	// Try to load existing cookies.
	if err := store.Load(); err == nil {
		logger.Info("cookie store initialized",
			slog.String("file_path", config.CookieFile),
			slog.Int("cookies_loaded", store.Count()))
	} else {
		logger.Debug("no existing cookies found", slog.String("file_path", config.CookieFile))
	}

	client := &WalmartClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects automatically.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cookieStore:       store,
		rateLimiter:       time.NewTicker(config.RateLimit),
		ledgerRateLimiter: time.NewTicker(ledgerRate),
		maxRetries:        maxRetries,
		logger:            logger,
	}

	return client, nil
}

// InitializeFromCurl loads cookies from a curl command file.
func (c *WalmartClient) InitializeFromCurl(curlFile string) error {
	// The curl file path is supplied by the caller of this library, not by
	// untrusted input, so reading it directly is intended behavior.
	// #nosec G304 -- caller-controlled configuration is the public API contract.
	data, err := os.ReadFile(curlFile)
	if err != nil {
		return fmt.Errorf("failed to read curl file: %w", err)
	}

	capture := cookies.ParseCurl(string(data))
	if len(capture.Cookies) == 0 {
		return fmt.Errorf("curl capture contains no cookies")
	}
	for _, name := range []string{cookieNameCID, cookieNameSPID, cookieNameAuth} {
		if capture.Cookies[name] == "" {
			return fmt.Errorf("curl capture is missing required %s cookie", name)
		}
	}

	// Mark essential cookies.
	essentialCookies := cookies.Essential()
	replacement := make(map[string]*cookies.Cookie, len(capture.Cookies))
	now := time.Now()

	for name, value := range capture.Cookies {
		cookie := &cookies.Cookie{
			Value:      value,
			LastUpdate: now,
			Source:     "curl",
			Essential:  false,
		}

		// Mark if essential.
		for _, essential := range essentialCookies {
			if name == essential {
				cookie.Essential = true
				break
			}
		}

		replacement[name] = cookie
	}

	profile := cookies.RequestProfile{Headers: make(map[string]string)}
	if match := getOrderHashPattern.FindStringSubmatch(capture.URL); len(match) >= 2 {
		profile.GetOrderHash = match[1]
	}
	for name, value := range capture.Headers {
		if _, ok := requestProfileHeaderAllowlist[name]; ok {
			profile.Headers[name] = value
		}
	}

	// A browser refresh is a coherent session snapshot. Replace instead of
	// merging so expired WAF cookies from an older capture cannot survive.
	c.cookieStore.Replace(replacement, profile)

	// Auto-save.
	if err := c.cookieStore.Save(); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}

	return nil
}

// Status shows the current state of cookies.
func (c *WalmartClient) Status() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allCookies := c.cookieStore.GetAll()

	fmt.Println("\n=== Cookie Store Status ===")
	fmt.Printf("Total cookies: %d\n", c.cookieStore.Count())
	fmt.Printf("Cookie file: %s\n", c.cookieStore.FilePath)
	fmt.Printf("Last update: %s\n", c.cookieStore.LastUpdate.Format(time.RFC3339))

	// Count by source.
	sources := make(map[string]int)
	essential := 0
	stale := 0

	for _, cookie := range allCookies {
		sources[cookie.Source]++
		if cookie.Essential {
			essential++
		}
		// Consider cookies older than 1 hour as potentially stale.
		if time.Since(cookie.LastUpdate) > time.Hour {
			stale++
		}
	}

	// Log warnings for stale cookies.
	if stale > 0 {
		c.logger.Warn("stale cookies detected",
			slog.Int("stale_count", stale),
			slog.Duration("age_threshold", time.Hour))
	}

	fmt.Printf("\nEssential cookies: %d\n", essential)
	fmt.Printf("Potentially stale: %d (>1 hour old)\n", stale)

	fmt.Println("\nCookies by source:")
	for source, count := range sources {
		fmt.Printf("  %s: %d\n", source, count)
	}

	// Show essential cookies status.
	fmt.Println("\nEssential cookies:")
	essentials := []string{cookieNameCID, cookieNameSPID, cookieNameAuth, "customer"}
	missingEssential := []string{}
	for _, name := range essentials {
		if cookie := c.cookieStore.Get(name); cookie != nil {
			age := time.Since(cookie.LastUpdate)
			status := "✅"
			if age > time.Hour {
				status = "⚠️"
			}
			fmt.Printf("  %s %s: %s ago\n", status, name, age.Round(time.Second))
		} else {
			missingEssential = append(missingEssential, name)
			fmt.Printf("  ❌ %s: MISSING\n", name)
		}
	}

	// Log warning for missing essential cookies.
	if len(missingEssential) > 0 {
		c.logger.Warn("missing essential cookies",
			slog.Any("missing_cookies", missingEssential))
	}
}

// defaultCurlPath is where RefreshFromBrowser tells the user to paste their
// captured cURL request. It lives in the repo root, which is gitignored
// (see .gitignore: "curl.txt"), so it's safe to reuse as a fixed drop point.
const defaultCurlPath = "curl.txt"

// RefreshFromBrowser walks the user through capturing a fresh 'getOrder'
// request from the browser and re-initializing cookies from it.
func (c *WalmartClient) RefreshFromBrowser() error {
	absDefault, err := filepath.Abs(defaultCurlPath)
	if err != nil {
		absDefault = defaultCurlPath
	}

	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│  Refresh Cookies from Browser                            │")
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  1. Open Chrome and go to: https://www.walmart.com/orders")
	fmt.Println("  2. Click 'View details' on any order")
	fmt.Println("  3. Open DevTools  (Cmd+Option+I / F12)")
	fmt.Println("  4. Click the 'Network' tab")
	fmt.Println("  5. Refresh the page  (Cmd+R / F5)")
	fmt.Println("  6. In the Network search bar, type:  getOrder")
	fmt.Println("  7. Right-click the request that appears → Copy → Copy as cURL")
	fmt.Printf("  8. Paste it into a file at:\n\n       %s\n\n", absDefault)
	fmt.Printf("Press Enter to use that path, or type a different one (or 'skip' to cancel)\n> ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		// Treat an empty line or read error as a cancellation.
		line = ""
	}
	path := strings.TrimSpace(line)

	if path == "skip" {
		return fmt.Errorf("refresh canceled")
	}
	if path == "" {
		path = defaultCurlPath
	}

	if err := c.InitializeFromCurl(path); err != nil {
		return err
	}

	fmt.Printf("\n✅ Cookies refreshed and saved.\n")
	return nil
}

// CookieCount returns the number of cookies currently stored.
func (c *WalmartClient) CookieCount() int {
	return c.cookieStore.Count()
}

// ExportCookies exports cookies to a simple JSON format (name -> value map).
func (c *WalmartClient) ExportCookies() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allCookies := c.cookieStore.GetAll()
	simple := make(map[string]string, len(allCookies))
	for name, cookie := range allCookies {
		simple[name] = cookie.Value
	}
	return simple
}

// SaveCookies persists the current cookie store to disk.
func (c *WalmartClient) SaveCookies() error {
	return c.cookieStore.Save()
}
