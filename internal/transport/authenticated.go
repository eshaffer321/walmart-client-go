package transport

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eshaffer321/walmart-client-go/internal/cookies"
)

// AuthenticatedTransport is an http.RoundTripper that automatically handles
// cookie-based authentication. It applies cookies to every outgoing request
// and updates the cookie store from incoming Set-Cookie headers.
//
// This implementation follows the standard Go middleware pattern using
// http.RoundTripper, making cookie auth impossible to forget and centralizing
// all authentication logic in one place.
type AuthenticatedTransport struct {
	base        http.RoundTripper
	cookieStore *cookies.Store
	mu          *sync.RWMutex
	logger      *slog.Logger
}

// NewAuthenticatedTransport creates a new authenticated transport that wraps
// the base RoundTripper with automatic cookie management.
func NewAuthenticatedTransport(store *cookies.Store, mu *sync.RWMutex, logger *slog.Logger) *AuthenticatedTransport {
	if logger == nil {
		// No-op logger if none provided
		logger = slog.New(slog.NewTextHandler(nil, nil))
	}

	return &AuthenticatedTransport{
		base:        http.DefaultTransport,
		cookieStore: store,
		mu:          mu,
		logger:      logger,
	}
}

// SetBaseTransport sets the base RoundTripper. This is useful for testing
// to redirect requests to a test server while preserving cookie handling.
func (t *AuthenticatedTransport) SetBaseTransport(base http.RoundTripper) {
	t.base = base
}

// RoundTrip implements http.RoundTripper. It automatically:
// 1. Applies cookies to the outgoing request
// 2. Executes the HTTP request
// 3. Updates cookies from the response Set-Cookie headers
func (t *AuthenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Apply cookies before every request
	t.applyCookies(req)

	// Execute the request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Update cookies from response
	t.updateCookies(resp)

	return resp, nil
}

// applyCookies adds all cookies from the store to the outgoing request
func (t *AuthenticatedTransport) applyCookies(req *http.Request) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	allCookies := t.cookieStore.GetAll()
	var cookiePairs []string
	for name, cookie := range allCookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", name, cookie.Value))
	}

	if len(cookiePairs) > 0 {
		req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
		t.logger.Debug("applied cookies to request",
			slog.Int("cookie_count", len(cookiePairs)),
			slog.String("url", req.URL.Path))
	}
}

// updateCookies updates the cookie store with Set-Cookie headers from the response
func (t *AuthenticatedTransport) updateCookies(resp *http.Response) {
	setCookies := resp.Header["Set-Cookie"]
	if len(setCookies) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	updatedCount := 0
	for _, cookieHeader := range setCookies {
		parts := strings.Split(cookieHeader, ";")
		if len(parts) > 0 {
			nameValue := strings.SplitN(parts[0], "=", 2)
			if len(nameValue) == 2 {
				name := strings.TrimSpace(nameValue[0])
				value := strings.TrimSpace(nameValue[1])

				// Check if this is an update to an existing cookie
				existing := t.cookieStore.Get(name)
				isUpdate := existing != nil && existing.Value != value

				if isUpdate {
					updatedCount++
				}

				// Store the cookie, preserving Essential flag if it exists
				t.cookieStore.Set(name, &cookies.Cookie{
					Value:      value,
					LastUpdate: time.Now(),
					Source:     "response",
					Essential:  existing != nil && existing.Essential,
				})
			}
		}
	}

	if updatedCount > 0 {
		t.logger.Debug("cookies updated from response",
			slog.Int("updated_count", updatedCount),
			slog.String("url", resp.Request.URL.Path))
	}
}
