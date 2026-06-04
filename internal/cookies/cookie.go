package cookies

import (
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

// ExtractFromCurl parses cookies from a curl command string.
func ExtractFromCurl(curlCmd string) map[string]string {
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

// Essential returns list of essential cookie names for authentication.
func Essential() []string {
	return []string{"CID", "SPID", "auth", "customer", "hasCID", "type"}
}
