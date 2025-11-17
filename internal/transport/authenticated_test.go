package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eshaffer321/walmart-client-go/internal/cookies"
)

func TestAuthenticatedTransport_AppliesCookies(t *testing.T) {
	// Create a test server that checks for cookies
	cookieReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		if strings.Contains(cookie, "test_cookie=test_value") {
			cookieReceived = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create cookie store with test cookie
	store := cookies.NewStore("", nil)
	store.Set("test_cookie", &cookies.Cookie{
		Value:      "test_value",
		LastUpdate: time.Now(),
		Source:     "test",
	})

	// Create authenticated transport
	var mu sync.RWMutex
	transport := NewAuthenticatedTransport(store, &mu, nil)

	// Make a request
	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify cookie was sent
	if !cookieReceived {
		t.Error("Cookie was not sent with request")
	}
}

func TestAuthenticatedTransport_UpdatesCookies(t *testing.T) {
	// Create a test server that sends Set-Cookie header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "new_cookie=new_value; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create empty cookie store
	store := cookies.NewStore("", nil)
	var mu sync.RWMutex
	transport := NewAuthenticatedTransport(store, &mu, nil)

	// Make a request
	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify cookie was stored
	cookie := store.Get("new_cookie")
	if cookie == nil {
		t.Fatal("Cookie was not stored")
	}

	if cookie.Value != "new_value" {
		t.Errorf("Expected cookie value 'new_value', got '%s'", cookie.Value)
	}

	if cookie.Source != "response" {
		t.Errorf("Expected cookie source 'response', got '%s'", cookie.Source)
	}
}

func TestAuthenticatedTransport_PreservesEssentialFlag(t *testing.T) {
	// Create a test server that updates an essential cookie
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "essential_cookie=updated_value; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create cookie store with essential cookie
	store := cookies.NewStore("", nil)
	store.Set("essential_cookie", &cookies.Cookie{
		Value:      "original_value",
		LastUpdate: time.Now(),
		Source:     "test",
		Essential:  true,
	})

	var mu sync.RWMutex
	transport := NewAuthenticatedTransport(store, &mu, nil)

	// Make a request
	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify Essential flag was preserved
	cookie := store.Get("essential_cookie")
	if cookie == nil {
		t.Fatal("Cookie was not found")
	}

	if !cookie.Essential {
		t.Error("Essential flag was not preserved during update")
	}

	if cookie.Value != "updated_value" {
		t.Errorf("Expected cookie value 'updated_value', got '%s'", cookie.Value)
	}
}

func TestAuthenticatedTransport_MultipleCookies(t *testing.T) {
	// Create a test server that checks for multiple cookies
	cookiesReceived := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		if strings.Contains(cookie, "cookie1=value1") {
			cookiesReceived["cookie1"] = true
		}
		if strings.Contains(cookie, "cookie2=value2") {
			cookiesReceived["cookie2"] = true
		}
		if strings.Contains(cookie, "cookie3=value3") {
			cookiesReceived["cookie3"] = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create cookie store with multiple cookies
	store := cookies.NewStore("", nil)
	store.Set("cookie1", &cookies.Cookie{Value: "value1", Source: "test"})
	store.Set("cookie2", &cookies.Cookie{Value: "value2", Source: "test"})
	store.Set("cookie3", &cookies.Cookie{Value: "value3", Source: "test"})

	var mu sync.RWMutex
	transport := NewAuthenticatedTransport(store, &mu, nil)

	// Make a request
	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify all cookies were sent
	if len(cookiesReceived) != 3 {
		t.Errorf("Expected 3 cookies to be sent, got %d", len(cookiesReceived))
	}
}

func TestAuthenticatedTransport_ThreadSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "test=value; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := cookies.NewStore("", nil)
	var mu sync.RWMutex
	transport := NewAuthenticatedTransport(store, &mu, nil)
	client := &http.Client{Transport: transport}

	// Make concurrent requests to test thread safety
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err != nil {
				t.Errorf("Request failed: %v", err)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// If we got here without panicking or deadlocking, test passes
}
