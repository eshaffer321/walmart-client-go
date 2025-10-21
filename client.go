package walmart

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/eshaffer321/walmart-client/internal/cookies"
)

// WalmartClient provides access to Walmart's order history and purchase data API
type WalmartClient struct {
	httpClient  *http.Client
	cookieStore *cookies.Store
	rateLimiter *time.Ticker
	lastRequest time.Time
	mu          sync.RWMutex
	logger      *slog.Logger
}

// NewWalmartClient creates a new Walmart API client with the given configuration
func NewWalmartClient(config ClientConfig) (*WalmartClient, error) {
	// Set defaults
	if config.CookieFile == "" {
		if config.CookieDir == "" {
			homeDir, _ := os.UserHomeDir()
			config.CookieDir = filepath.Join(homeDir, ".walmart-api")
		}
		_ = os.MkdirAll(config.CookieDir, 0755)
		config.CookieFile = filepath.Join(config.CookieDir, "cookies.json")
	}

	if config.RateLimit == 0 {
		config.RateLimit = 2 * time.Second
	}

	// Initialize logger with client attribute
	logger := config.Logger
	if logger == nil {
		// Use no-op logger that discards all output
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	logger = logger.With(slog.String("client", "walmart"))

	// Initialize cookie store
	store := cookies.NewStore(config.CookieFile, logger)

	// Try to load existing cookies
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
			// Don't follow redirects automatically
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cookieStore: store,
		rateLimiter: time.NewTicker(config.RateLimit),
		logger:      logger,
	}

	return client, nil
}

// InitializeFromCurl loads cookies from a curl command file
func (c *WalmartClient) InitializeFromCurl(curlFile string) error {
	data, err := os.ReadFile(curlFile)
	if err != nil {
		return fmt.Errorf("failed to read curl file: %w", err)
	}

	parsedCookies := cookies.ExtractFromCurl(string(data))

	c.mu.Lock()
	defer c.mu.Unlock()

	// Mark essential cookies
	essentialCookies := cookies.Essential()

	for name, value := range parsedCookies {
		cookie := &cookies.Cookie{
			Value:      value,
			LastUpdate: time.Now(),
			Source:     "curl",
			Essential:  false,
		}

		// Mark if essential
		for _, essential := range essentialCookies {
			if name == essential {
				cookie.Essential = true
				break
			}
		}

		c.cookieStore.Set(name, cookie)
	}

	// Auto-save
	if err := c.cookieStore.Save(); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}

	return nil
}

// Status shows the current state of cookies
func (c *WalmartClient) Status() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allCookies := c.cookieStore.GetAll()

	fmt.Println("\n=== Cookie Store Status ===")
	fmt.Printf("Total cookies: %d\n", c.cookieStore.Count())
	fmt.Printf("Cookie file: %s\n", c.cookieStore.FilePath)
	fmt.Printf("Last update: %s\n", c.cookieStore.LastUpdate.Format(time.RFC3339))

	// Count by source
	sources := make(map[string]int)
	essential := 0
	stale := 0

	for _, cookie := range allCookies {
		sources[cookie.Source]++
		if cookie.Essential {
			essential++
		}
		// Consider cookies older than 1 hour as potentially stale
		if time.Since(cookie.LastUpdate) > time.Hour {
			stale++
		}
	}

	// Log warnings for stale cookies
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

	// Show essential cookies status
	fmt.Println("\nEssential cookies:")
	essentials := []string{"CID", "SPID", "auth", "customer"}
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

	// Log warning for missing essential cookies
	if len(missingEssential) > 0 {
		c.logger.Warn("missing essential cookies",
			slog.Any("missing_cookies", missingEssential))
	}
}

// RefreshFromBrowser prompts user to get fresh cookies
func (c *WalmartClient) RefreshFromBrowser() error {
	fmt.Println("\n=== Refresh Cookies from Browser ===")
	fmt.Println("1. Open Chrome/Firefox and log into walmart.com")
	fmt.Println("2. Go to your orders page")
	fmt.Println("3. Open DevTools (F12) → Network tab")
	fmt.Println("4. Refresh the page")
	fmt.Println("5. Find any 'getOrder' request")
	fmt.Println("6. Right-click → Copy → Copy as cURL")
	fmt.Println("7. Paste into a file and provide the path below")
	fmt.Print("\nPath to curl file (or 'skip' to cancel): ")

	var path string
	_, _ = fmt.Scanln(&path)

	if path == "skip" || path == "" {
		return fmt.Errorf("refresh cancelled")
	}

	return c.InitializeFromCurl(path)
}

// CookieCount returns the number of cookies currently stored
func (c *WalmartClient) CookieCount() int {
	return c.cookieStore.Count()
}

// ExportCookies exports cookies to a simple JSON format (name -> value map)
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

// SaveCookies persists the current cookie store to disk
func (c *WalmartClient) SaveCookies() error {
	return c.cookieStore.Save()
}
