package cookies

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RequestProfile stores the browser request fingerprint captured alongside
// Walmart session cookies. Headers are intentionally limited by the caller to
// non-cookie values that are safe and useful to replay.
type RequestProfile struct {
	GetOrderHash string            `json:"get_order_hash,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// Store manages cookies with persistence and auto-updates.
type Store struct {
	Cookies        map[string]*Cookie `json:"cookies"`
	LastUpdate     time.Time          `json:"last_update"`
	RequestProfile *RequestProfile    `json:"request_profile,omitempty"`
	FilePath       string             `json:"-"`
	mu             sync.RWMutex
	logger         *slog.Logger
}

// NewStore creates a new cookie store.
func NewStore(filePath string, logger *slog.Logger) *Store {
	return &Store{
		Cookies:  make(map[string]*Cookie),
		FilePath: filePath,
		logger:   logger,
	}
}

// Get retrieves a cookie by name (thread-safe).
func (s *Store) Get(name string) *Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Cookies[name]
}

// Set updates or adds a cookie (thread-safe).
func (s *Store) Set(name string, cookie *Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cookies[name] = cookie
	s.LastUpdate = time.Now()
}

// Replace atomically swaps the current browser cookie snapshot and request
// profile in memory. It is used for explicit browser refreshes so cookies from
// older captures cannot leak into the new session.
func (s *Store) Replace(replacement map[string]*Cookie, profile RequestProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Cookies = cloneCookies(replacement)
	profileCopy := cloneRequestProfile(profile)
	s.RequestProfile = &profileCopy
	s.LastUpdate = time.Now()
}

// GetRequestProfile returns a copy of the captured browser request profile.
func (s *Store) GetRequestProfile() RequestProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.RequestProfile == nil {
		return RequestProfile{}
	}
	return cloneRequestProfile(*s.RequestProfile)
}

// Load reads cookies from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logger != nil {
		s.logger.Debug("loading cookies", slog.String("file_path", s.FilePath))
	}

	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Debug("failed to load cookies", slog.String("error", err.Error()))
		}
		return err
	}

	if err := json.Unmarshal(data, s); err != nil {
		if s.logger != nil {
			s.logger.Error("failed to parse cookie file",
				slog.String("file_path", s.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}
	if s.Cookies == nil {
		s.Cookies = make(map[string]*Cookie)
	}

	if s.logger != nil {
		s.logger.Debug("cookies loaded",
			slog.String("file_path", s.FilePath),
			slog.Int("count", len(s.Cookies)))
	}

	return nil
}

// Save writes cookies to disk.
func (s *Store) Save() error {
	s.mu.RLock()
	if s.logger != nil {
		s.logger.Debug("saving cookies",
			slog.String("file_path", s.FilePath),
			slog.Int("count", len(s.Cookies)))
	}

	data, err := json.MarshalIndent(s, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to marshal cookies",
				slog.String("file_path", s.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	// Write to a new owner-only file and rename it into place. Besides avoiding
	// partially written JSON, replacement fixes permissions on legacy files
	// that were previously created as group/world-readable.
	dir := filepath.Dir(s.FilePath)
	tmp, err := os.CreateTemp(dir, ".cookies-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary cookie file: %w", err)
	}
	tmpPath := tmp.Name()
	tmpClosed := false
	defer func() {
		if !tmpClosed {
			if closeErr := tmp.Close(); closeErr != nil && s.logger != nil {
				s.logger.Debug("failed to close temporary cookie file", slog.String("error", closeErr.Error()))
			}
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && s.logger != nil {
			s.logger.Debug("failed to remove temporary cookie file", slog.String("error", removeErr.Error()))
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		return fmt.Errorf("secure temporary cookie file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temporary cookie file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary cookie file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary cookie file: %w", err)
	}
	tmpClosed = true
	if err := os.Rename(tmpPath, s.FilePath); err != nil {
		if s.logger != nil {
			s.logger.Error("failed to write cookie file",
				slog.String("file_path", s.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}
	if err := os.Chmod(s.FilePath, 0600); err != nil {
		return fmt.Errorf("secure cookie file: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug("cookies saved", slog.String("file_path", s.FilePath))
	}

	return nil
}

// Count returns the number of cookies in the store.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Cookies)
}

// GetAll returns a copy of all cookies (thread-safe).
func (s *Store) GetAll() map[string]*Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]*Cookie, len(s.Cookies))
	for k, v := range s.Cookies {
		snapshot[k] = v
	}
	return snapshot
}

func cloneCookies(source map[string]*Cookie) map[string]*Cookie {
	clone := make(map[string]*Cookie, len(source))
	for name, cookie := range source {
		if cookie == nil {
			continue
		}
		cookieCopy := *cookie
		clone[name] = &cookieCopy
	}
	return clone
}

func cloneRequestProfile(profile RequestProfile) RequestProfile {
	clone := RequestProfile{
		GetOrderHash: profile.GetOrderHash,
		Headers:      make(map[string]string, len(profile.Headers)),
	}
	for name, value := range profile.Headers {
		clone.Headers[name] = value
	}
	return clone
}
