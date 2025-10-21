package cookies

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Store manages cookies with persistence and auto-updates
type Store struct {
	Cookies    map[string]*Cookie `json:"cookies"`
	LastUpdate time.Time          `json:"last_update"`
	FilePath   string             `json:"-"`
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewStore creates a new cookie store
func NewStore(filePath string, logger *slog.Logger) *Store {
	return &Store{
		Cookies:  make(map[string]*Cookie),
		FilePath: filePath,
		logger:   logger,
	}
}

// Get retrieves a cookie by name (thread-safe)
func (s *Store) Get(name string) *Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Cookies[name]
}

// Set updates or adds a cookie (thread-safe)
func (s *Store) Set(name string, cookie *Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cookies[name] = cookie
	s.LastUpdate = time.Now()
}

// Load reads cookies from disk
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

	if s.logger != nil {
		s.logger.Debug("cookies loaded",
			slog.String("file_path", s.FilePath),
			slog.Int("count", len(s.Cookies)))
	}

	return nil
}

// Save writes cookies to disk
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logger != nil {
		s.logger.Debug("saving cookies",
			slog.String("file_path", s.FilePath),
			slog.Int("count", len(s.Cookies)))
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to marshal cookies",
				slog.String("file_path", s.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	if err := os.WriteFile(s.FilePath, data, 0644); err != nil {
		if s.logger != nil {
			s.logger.Error("failed to write cookie file",
				slog.String("file_path", s.FilePath),
				slog.String("error", err.Error()))
		}
		return err
	}

	if s.logger != nil {
		s.logger.Debug("cookies saved", slog.String("file_path", s.FilePath))
	}

	return nil
}

// Count returns the number of cookies in the store
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Cookies)
}

// GetAll returns a copy of all cookies (thread-safe)
func (s *Store) GetAll() map[string]*Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]*Cookie, len(s.Cookies))
	for k, v := range s.Cookies {
		copy[k] = v
	}
	return copy
}
