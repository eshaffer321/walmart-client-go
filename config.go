package walmart

import (
	"log/slog"
	"time"
)

// ClientConfig configures the WalmartClient
type ClientConfig struct {
	// CookieFile specifies the path to the cookie storage file.
	// If empty, defaults to ~/.walmart-api/cookies.json
	CookieFile string `json:"cookie_file"`

	// RateLimit specifies the delay between API requests.
	// If zero, defaults to 2 seconds.
	RateLimit time.Duration `json:"rate_limit"`

	// AutoSave automatically persists cookies to disk after updates.
	AutoSave bool `json:"auto_save"`

	// CookieDir specifies the directory for cookie storage.
	// If empty and CookieFile is also empty, defaults to ~/.walmart-api/
	CookieDir string `json:"cookie_dir"`

	// Logger for structured logging. If nil, logging is disabled.
	// All logs include a "client=walmart" attribute for filtering.
	Logger *slog.Logger `json:"-"`
}
