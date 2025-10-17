package walmart

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WalmartClient is a robust client with automatic cookie management
type WalmartClient struct {
	httpClient  *http.Client
	CookieStore *CookieStore
	rateLimiter *time.Ticker
	lastRequest time.Time
	mu          sync.RWMutex
	logger      *slog.Logger
}

// CookieStore manages cookies with persistence and auto-updates
type CookieStore struct {
	Cookies    map[string]*Cookie `json:"cookies"`
	LastUpdate time.Time          `json:"last_update"`
	FilePath   string             `json:"-"`
	mu         sync.RWMutex
	logger     *slog.Logger
}

// Cookie represents a cookie with metadata
type Cookie struct {
	Value      string    `json:"value"`
	LastUpdate time.Time `json:"last_update"`
	Source     string    `json:"source"` // "curl", "response", "manual"
	Essential  bool      `json:"essential"`
}

// ClientConfig for initializing the client
type ClientConfig struct {
	CookieFile string        `json:"cookie_file"`
	RateLimit  time.Duration `json:"rate_limit"`
	AutoSave   bool          `json:"auto_save"`
	CookieDir  string        `json:"cookie_dir"`
	Logger     *slog.Logger  `json:"-"`
}

// NewWalmartClient creates a robust client with cookie management
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
		logger = slog.Default()
	}
	logger = logger.With(slog.String("client", "walmart"))

	// Initialize cookie store
	store := &CookieStore{
		Cookies:  make(map[string]*Cookie),
		FilePath: config.CookieFile,
		logger:   logger,
	}

	// Try to load existing cookies
	if err := store.Load(); err == nil {
		logger.Info("cookie store initialized",
			slog.String("file_path", config.CookieFile),
			slog.Int("cookies_loaded", len(store.Cookies)))
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
		CookieStore: store,
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

	cookies := extractCookiesFromCurl(string(data))

	c.mu.Lock()
	defer c.mu.Unlock()

	// Mark essential cookies
	essentialCookies := []string{"CID", "SPID", "auth", "customer", "hasCID", "type"}

	for name, value := range cookies {
		cookie := &Cookie{
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

		c.CookieStore.Set(name, cookie)
	}

	// Auto-save
	if err := c.CookieStore.Save(); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}

	return nil
}

// GetOrder fetches an order with automatic cookie updates
func (c *WalmartClient) GetOrder(orderID string, isInStore bool) (*Order, error) {
	// Rate limiting - only wait if not first request
	if !c.lastRequest.IsZero() {
		c.logger.Debug("waiting for rate limiter")
		<-c.rateLimiter.C
	}
	c.lastRequest = time.Now()

	endpoint := c.buildOrderEndpoint(orderID, isInStore)

	c.logger.Debug("fetching order",
		slog.String("order_id", orderID),
		slog.Bool("is_in_store", isInStore),
		slog.String("endpoint", endpoint))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		c.logger.Error("failed to create request",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	c.setHeaders(req)

	// Set cookies from store
	c.setCookies(req)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("request failed",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.logger.Debug("received response",
		slog.String("order_id", orderID),
		slog.Int("status_code", resp.StatusCode))

	// Update cookies from response
	c.updateCookiesFromResponse(resp)

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read response",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("response received",
		slog.String("order_id", orderID),
		slog.Int("response_size", len(body)))

	// Check status
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limited",
				slog.String("order_id", orderID),
				slog.Int("status_code", resp.StatusCode))
			return nil, fmt.Errorf("rate limited - cookies might be stale, try refreshing from browser")
		}
		if resp.StatusCode == 403 || resp.StatusCode == 418 {
			c.logger.Warn("access denied",
				slog.String("order_id", orderID),
				slog.Int("status_code", resp.StatusCode))
			return nil, fmt.Errorf("access denied - cookies expired, please update from browser")
		}
		c.logger.Warn("non-200 response",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		c.logger.Error("failed to parse response",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if orderResp.Data.Order == nil {
		c.logger.Error("no order data in response", slog.String("order_id", orderID))
		return nil, fmt.Errorf("no order data in response")
	}

	order := orderResp.Data.Order

	// Calculate total with tip for delivery orders
	if order.IsDeliveryOrder() {
		order.CalculateTotalWithTip()
	}

	// Log order fetch with appropriate details
	total := 0.0
	if order.PriceDetails != nil && order.PriceDetails.GrandTotal != nil {
		total = order.PriceDetails.GrandTotal.Value
	}

	c.logger.Info("fetched order",
		slog.String("order_id", orderID),
		slog.Bool("is_in_store", isInStore),
		slog.Float64("total", total),
		slog.Int("item_count", order.GetItemCount()))

	// Auto-save cookies after successful request
	_ = c.CookieStore.Save()

	return order, nil
}

// GetOrderAutoDetect tries to fetch an order, automatically detecting if it's in-store or delivery
func (c *WalmartClient) GetOrderAutoDetect(orderID string) (*Order, error) {
	// First try as in-store (most common for the user's examples)
	order, err := c.GetOrder(orderID, true)
	if err == nil {
		return order, nil
	}

	// If that fails, try as delivery order
	order, err = c.GetOrder(orderID, false)
	if err == nil {
		return order, nil
	}

	return nil, fmt.Errorf("order not found as either in-store or delivery: %w", err)
}

// GetDeliveryOrderWithTip fetches a delivery order and ensures tip information is included
func (c *WalmartClient) GetDeliveryOrderWithTip(orderID string) (*Order, error) {
	// Fetch as delivery order (isInStore = false)
	order, err := c.GetOrder(orderID, false)
	if err != nil {
		return nil, err
	}

	// Calculate total with tip if not already present
	if order.PriceDetails != nil && order.PriceDetails.TotalWithTip == nil {
		order.CalculateTotalWithTip()
	}

	return order, nil
}

// updateCookiesFromResponse updates cookie store with Set-Cookie headers
func (c *WalmartClient) updateCookiesFromResponse(resp *http.Response) {
	setCookies := resp.Header["Set-Cookie"]
	if len(setCookies) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	updatedCount := 0
	for _, cookieHeader := range setCookies {
		parts := strings.Split(cookieHeader, ";")
		if len(parts) > 0 {
			nameValue := strings.SplitN(parts[0], "=", 2)
			if len(nameValue) == 2 {
				name := strings.TrimSpace(nameValue[0])
				value := strings.TrimSpace(nameValue[1])

				// Check if this is an update
				existing := c.CookieStore.Get(name)
				if existing != nil && existing.Value != value {
					updatedCount++
				}

				c.CookieStore.Set(name, &Cookie{
					Value:      value,
					LastUpdate: time.Now(),
					Source:     "response",
					Essential:  existing != nil && existing.Essential,
				})
			}
		}
	}

	if updatedCount > 0 {
		c.logger.Info("cookies updated from response",
			slog.Int("updated_count", updatedCount))
	}
}

// Status shows the current state of cookies
func (c *WalmartClient) Status() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fmt.Println("\n=== Cookie Store Status ===")
	fmt.Printf("Total cookies: %d\n", len(c.CookieStore.Cookies))
	fmt.Printf("Cookie file: %s\n", c.CookieStore.FilePath)
	fmt.Printf("Last update: %s\n", c.CookieStore.LastUpdate.Format(time.RFC3339))

	// Count by source
	sources := make(map[string]int)
	essential := 0
	stale := 0

	for _, cookie := range c.CookieStore.Cookies {
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
		if cookie := c.CookieStore.Get(name); cookie != nil {
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

// Cookie Store Methods

func (cs *CookieStore) Get(name string) *Cookie {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.Cookies[name]
}

func (cs *CookieStore) Set(name string, cookie *Cookie) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.Cookies[name] = cookie
	cs.LastUpdate = time.Now()
}

func (cs *CookieStore) Load() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.logger != nil {
		cs.logger.Debug("loading cookies", slog.String("file_path", cs.FilePath))
	}

	data, err := os.ReadFile(cs.FilePath)
	if err != nil {
		if cs.logger != nil {
			cs.logger.Debug("failed to load cookies", slog.String("error", err.Error()))
		}
		return err
	}

	if err := json.Unmarshal(data, cs); err != nil {
		if cs.logger != nil {
			cs.logger.Error("failed to parse cookie file",
				slog.String("file_path", cs.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	if cs.logger != nil {
		cs.logger.Debug("cookies loaded",
			slog.String("file_path", cs.FilePath),
			slog.Int("count", len(cs.Cookies)))
	}

	return nil
}

func (cs *CookieStore) Save() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.logger != nil {
		cs.logger.Debug("saving cookies",
			slog.String("file_path", cs.FilePath),
			slog.Int("count", len(cs.Cookies)))
	}

	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		if cs.logger != nil {
			cs.logger.Error("failed to marshal cookies",
				slog.String("file_path", cs.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	if err := os.WriteFile(cs.FilePath, data, 0644); err != nil {
		if cs.logger != nil {
			cs.logger.Error("failed to write cookie file",
				slog.String("file_path", cs.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	if cs.logger != nil {
		cs.logger.Debug("cookies saved", slog.String("file_path", cs.FilePath))
	}

	return nil
}

// Helper functions

func (c *WalmartClient) buildOrderEndpoint(orderID string, isInStore bool) string {
	variables := map[string]interface{}{
		"orderId":              orderID,
		"orderIsInStore":       isInStore,
		"clickThroughGroupId":  "0",
		"enableIsWcpOrder":     false,
		"enabledFeatures":      []string{"csat-northstar-v1", "tips", "delivery-fees"},
		"enableSignOnDelivery": true,
		"includeTipDetails":    true,
		"includeFeesDetails":   true,
	}

	variablesJSON, _ := json.Marshal(variables)
	params := url.Values{}
	params.Set("variables", string(variablesJSON))

	return fmt.Sprintf("https://www.walmart.com/orchestra/orders/graphql/getOrder/d0622497daef19150438d07c506739d451cad6749cf45c3b4db95f2f5a0a65c4?%s",
		params.Encode())
}

func (c *WalmartClient) setHeaders(req *http.Request) {
	headers := map[string]string{
		"accept":                  "application/json",
		"accept-language":         "en-US",
		"content-type":            "application/json",
		"user-agent":              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
		"x-apollo-operation-name": "getOrder",
		"x-o-gql-query":           "query getOrder",
		"x-o-platform":            "rweb",
		"x-o-bu":                  "WALMART-US",
		"x-o-mart":                "B2C",
		"x-o-segment":             "oaoh",
		"x-o-correlation-id":      fmt.Sprintf("walmart-go-%d", time.Now().Unix()),
		"wm_qos.correlation_id":   fmt.Sprintf("walmart-go-%d", time.Now().Unix()),
		"wm_mp":                   "true",
		"sec-fetch-site":          "same-origin",
		"sec-fetch-mode":          "cors",
		"sec-fetch-dest":          "empty",
		"dnt":                     "1",
		"x-o-platform-version":    "usweb-1.221.0",
		"x-enable-server-timing":  "1",
		"x-latency-trace":         "1",
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func (c *WalmartClient) setCookies(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var cookiePairs []string
	for name, cookie := range c.CookieStore.Cookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", name, cookie.Value))
	}

	if len(cookiePairs) > 0 {
		req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
	}
}

func extractCookiesFromCurl(curlCmd string) map[string]string {
	cookies := make(map[string]string)
	lines := strings.Split(curlCmd, "\\\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-b '") || strings.HasPrefix(line, "--cookie '") {
			start := strings.Index(line, "'") + 1
			end := strings.LastIndex(line, "'")
			if start > 0 && end > start {
				cookieString := line[start:end]
				pairs := strings.Split(cookieString, "; ")
				for _, pair := range pairs {
					parts := strings.SplitN(pair, "=", 2)
					if len(parts) == 2 {
						cookies[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return cookies
}
