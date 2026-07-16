package cookies

import (
	"regexp"
	"strings"
	"time"
)

// Cookie represents a cookie with metadata for tracking updates and source.
type Cookie struct {
	Value      string    `json:"value"`
	LastUpdate time.Time `json:"last_update"`
	Source     string    `json:"source"` // "curl", "response", "manual".
	Essential  bool      `json:"essential"`
}

const (
	cookieNameAuth = "auth"
	cookieNameCID  = "CID"
	cookieNameSPID = "SPID"
)

// CurlCapture contains the request metadata that can be safely reused from a
// browser's "Copy as cURL" output. Cookie credentials are kept separate from
// ordinary headers so callers cannot accidentally persist them as profile
// headers or log them with request diagnostics.
type CurlCapture struct {
	URL     string
	Headers map[string]string
	Cookies map[string]string
}

var (
	curlURLPattern    = regexp.MustCompile(`(?m)\bcurl\s+'([^']+)'`)
	curlHeaderPattern = regexp.MustCompile(`(?m)(?:^|\s)(?:-H|--header)\s+'([^']+)'`)
	curlCookiePattern = regexp.MustCompile(`(?m)(?:^|\s)(?:-b|--cookie)\s+'([^']*)'`)
)

// ParseCurl parses the URL, headers, and cookies from a Chromium-style
// "Copy as cURL (bash)" command. Header names are normalized to lower case.
func ParseCurl(curlCmd string) CurlCapture {
	capture := CurlCapture{
		Headers: make(map[string]string),
		Cookies: make(map[string]string),
	}

	if match := curlURLPattern.FindStringSubmatch(curlCmd); len(match) == 2 {
		capture.URL = match[1]
	}

	for _, match := range curlHeaderPattern.FindAllStringSubmatch(curlCmd, -1) {
		if len(match) != 2 {
			continue
		}
		name, value, ok := strings.Cut(match[1], ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if name == "cookie" {
			parseCookiePairs(value, capture.Cookies)
			continue
		}
		capture.Headers[name] = value
	}

	for _, match := range curlCookiePattern.FindAllStringSubmatch(curlCmd, -1) {
		if len(match) == 2 {
			parseCookiePairs(match[1], capture.Cookies)
		}
	}

	return capture
}

// ExtractFromCurl parses cookies from a curl command string.
func ExtractFromCurl(curlCmd string) map[string]string {
	return ParseCurl(curlCmd).Cookies
}

func parseCookiePairs(cookieString string, target map[string]string) {
	for _, pair := range strings.Split(cookieString, ";") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		target[name] = strings.TrimSpace(value)
	}
}

// Essential returns list of essential cookie names for authentication.
func Essential() []string {
	return []string{cookieNameCID, cookieNameSPID, cookieNameAuth, "customer", "hasCID", "type"}
}
