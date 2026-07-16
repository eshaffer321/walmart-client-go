package walmart

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/eshaffer321/walmart-client-go/v2/internal/cookies"
)

const contentTypeJSON = "application/json"

const (
	cookieNameAuth         = "auth"
	cookieNameCID          = "CID"
	cookieNameSPID         = "SPID"
	defaultGetOrderHash    = "0d0e73dcfbe4c7a4cb6c8ce929e5b9a8b3e731e4bac81969eed76dbdab28b0d2"
	defaultPlatform        = "usweb-1.284.0-1871dae08edcd429b12e47a369da9967df5b330d-7141058r"
	defaultUserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	graphqlOrderIDVariable = "orderId"
	headerPlatformVersion  = "x-o-platform-version"
	headerUserAgent        = "user-agent"
)

// ErrBotChallenge identifies Walmart's non-standard HTTP 456 response. It
// means the request was rejected by bot protection and the browser session
// profile should be refreshed before retrying.
var ErrBotChallenge = errors.New("walmart rejected request as automated")

var requestProfileHeaderAllowlist = map[string]struct{}{
	"cache-control":         {},
	"device-memory":         {},
	"device_profile_ref_id": {},
	"downlink":              {},
	"dpr":                   {},
	"pragma":                {},
	"priority":              {},
	"sec-ch-device-memory":  {},
	"sec-ch-dpr":            {},
	"sec-ch-ua":             {},
	"sec-ch-ua-mobile":      {},
	"sec-ch-ua-platform":    {},
	"tenant-id":             {},
	headerUserAgent:         {},
	"x-o-ccm":               {},
	headerPlatformVersion:   {},
}

// GetOrder fetches an order with automatic cookie updates.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetOrder(ctx context.Context, orderID string, isInStore bool) (*Order, error) {
	return c.GetOrderWithGroup(ctx, orderID, "0", isInStore)
}

// GetOrderWithGroup fetches an order using the group identifier returned by
// purchase history. Walmart's web client includes this identifier in both the
// GraphQL variables and order-page request context.
func (c *WalmartClient) GetOrderWithGroup(ctx context.Context, orderID, groupID string, isInStore bool) (*Order, error) {
	// Rate limiting with context support.
	if !c.lastRequest.IsZero() {
		c.logger.Debug("waiting for rate limiter")
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request canceled while rate limiting: %w", ctx.Err())
		case <-c.rateLimiter.C:
			// Continue.
		}
	}
	c.lastRequest = time.Now()

	endpoint := c.buildOrderEndpointWithGroup(orderID, groupID, isInStore)

	c.logger.Debug("fetching order",
		slog.String("order_id", orderID),
		slog.Bool("is_in_store", isInStore),
		slog.String("endpoint", endpoint))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		c.logger.Error("failed to create request",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers.
	c.setHeaders(req, "getOrder", buildOrderPageURL(orderID, groupID))

	// Set cookies from store.
	c.setCookies(req)

	// Execute request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("request failed",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.logger.Debug("failed to close response body", slog.String("error", cerr.Error()))
		}
	}()

	c.logger.Debug("received response",
		slog.String("order_id", orderID),
		slog.Int("status_code", resp.StatusCode))

	// Update cookies from response.
	c.updateCookiesFromResponse(resp)

	// Read body.
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

	// Check status.
	if statusErr := c.checkOrderResponseError(resp, body, orderID); statusErr != nil {
		return nil, statusErr
	}

	// Parse response.
	order, err := c.parseOrderResponse(body, orderID)
	if err != nil {
		return nil, err
	}

	// Calculate total with tip for delivery orders.
	if order.IsDeliveryOrder() {
		order.CalculateTotalWithTip()
	}

	// Log order fetch with appropriate details.
	total := 0.0
	if order.PriceDetails != nil && order.PriceDetails.GrandTotal != nil {
		total = order.PriceDetails.GrandTotal.Value
	}

	c.logger.Info("fetched order",
		slog.String("order_id", orderID),
		slog.Bool("is_in_store", isInStore),
		slog.Float64("total", total),
		slog.Int("item_count", order.GetItemCount()))

	// Auto-save cookies after successful request.
	if err := c.cookieStore.Save(); err != nil {
		c.logger.Warn("failed to save cookies after request",
			slog.String("order_id", orderID),
			slog.String("error", err.Error()))
	}

	return order, nil
}

// parseOrderResponse unmarshals an order response body and returns the order,
// or an error when the payload is malformed or missing order data.
func (c *WalmartClient) parseOrderResponse(body []byte, orderID string) (*Order, error) {
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

	return orderResp.Data.Order, nil
}

// checkOrderResponseError returns an error describing a non-OK order response,
// or nil when the status is HTTP 200.
func (c *WalmartClient) checkOrderResponseError(resp *http.Response, body []byte, orderID string) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	switch resp.StatusCode {
	case 429:
		c.logger.Warn("rate limited",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("rate limited - cookies might be stale, try refreshing from browser")
	case 403, 418:
		c.logger.Warn("access denied",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("access denied - cookies expired, please update from browser")
	case 456:
		c.logger.Warn("bot challenge",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode))
		return botChallengeError(body)
	default:
		c.logger.Warn("non-200 response",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
}

// GetOrderAutoDetect tries to fetch an order, automatically detecting if it's in-store or delivery.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetOrderAutoDetect(ctx context.Context, orderID string) (*Order, error) {
	// First try as in-store (most common for the user's examples).
	order, err := c.GetOrder(ctx, orderID, true)
	if err == nil {
		return order, nil
	}
	if errors.Is(err, ErrBotChallenge) {
		return nil, err
	}

	// Check for cancellation before retrying.
	if ctx.Err() != nil {
		return nil, fmt.Errorf("request canceled: %w", ctx.Err())
	}

	// If that fails, try as delivery order.
	order, err = c.GetOrder(ctx, orderID, false)
	if err == nil {
		return order, nil
	}

	return nil, fmt.Errorf("order not found as either in-store or delivery: %w", err)
}

// GetDeliveryOrderWithTip fetches a delivery order and ensures tip information is included.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetDeliveryOrderWithTip(ctx context.Context, orderID string) (*Order, error) {
	// Fetch as delivery order (isInStore = false).
	order, err := c.GetOrder(ctx, orderID, false)
	if err != nil {
		return nil, err
	}

	// Calculate total with tip if not already present.
	if order.PriceDetails != nil && order.PriceDetails.TotalWithTip == nil {
		order.CalculateTotalWithTip()
	}

	return order, nil
}

// GetOrdersAsJSON is a helper that returns recent orders as JSON string.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetOrdersAsJSON(ctx context.Context, limit int) (string, error) {
	orders, err := c.GetRecentOrders(ctx, limit)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return string(jsonData), nil
}

// GetOrderAsJSON is a helper that returns order details as JSON string.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetOrderAsJSON(ctx context.Context, orderID string, isInStore bool) (string, error) {
	order, err := c.GetOrder(ctx, orderID, isInStore)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return string(jsonData), nil
}

// buildOrderEndpoint constructs the GraphQL endpoint URL for fetching order details.
func (c *WalmartClient) buildOrderEndpoint(orderID string, isInStore bool) string {
	return c.buildOrderEndpointWithGroup(orderID, "0", isInStore)
}

func (c *WalmartClient) buildOrderEndpointWithGroup(orderID, groupID string, isInStore bool) string {
	variables := map[string]interface{}{
		"clickThroughGroupId":       groupID,
		"eligibleFeatures":          map[string]bool{"isEbtEligible": false, "isLotOnOdpEnable": false},
		"enableCancelFix":           false,
		"enableGroupBannerMessages": false,
		"enableIsWcpOrder":          false,
		"enableSignOnDelivery":      true,
		"enableVolumePricing":       false,
		"enableWcpPhaseOrder":       false,
		"enabledFeatures":           []string{"csc", "csat-northstar-v1"},
		graphqlOrderIDVariable:      orderID,
		"orderIsInStore":            isInStore,
	}

	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		c.logger.Error("failed to marshal order variables", slog.String("error", err.Error()))
	}
	params := url.Values{}
	params.Set("variables", string(variablesJSON))

	hash := defaultGetOrderHash
	if profileHash := c.cookieStore.GetRequestProfile().GetOrderHash; profileHash != "" {
		hash = profileHash
	}

	return fmt.Sprintf("https://www.walmart.com/orchestra/orders/graphql/getOrder/%s?%s", hash, params.Encode())
}

// setHeaders sets standard HTTP headers required by Walmart's API.
func (c *WalmartClient) setHeaders(req *http.Request, operationName, pageURL string) {
	headers := map[string]string{
		"accept":                  contentTypeJSON,
		"accept-language":         "en-US,en;q=0.9",
		"content-type":            contentTypeJSON,
		headerUserAgent:           defaultUserAgent,
		"x-apollo-operation-name": operationName,
		"x-o-gql-query":           fmt.Sprintf("query %s", operationName),
		"x-o-platform":            "rweb",
		"x-o-bu":                  "WALMART-US",
		"x-o-mart":                "B2C",
		"x-o-segment":             "oaoh",
		"wm_mp":                   "true",
		"sec-fetch-site":          "same-origin",
		"sec-fetch-mode":          "cors",
		"sec-fetch-dest":          "empty",
		"sec-ch-ua-mobile":        "?0",
		"sec-ch-ua-platform":      `"macOS"`,
		headerPlatformVersion:     defaultPlatform,
		"x-enable-server-timing":  "1",
		"x-latency-trace":         "1",
	}

	profile := c.cookieStore.GetRequestProfile()
	for name, value := range profile.Headers {
		if _, ok := requestProfileHeaderAllowlist[name]; ok {
			headers[name] = value
		}
	}

	correlationID := randomUUID()
	headers["x-o-correlation-id"] = correlationID
	headers["wm_qos.correlation_id"] = correlationID
	headers["wm-client-traceid"] = randomHex(16)
	headers["traceparent"] = fmt.Sprintf("00-%s-%s-01", randomHex(16), randomHex(8))
	if pageURL != "" {
		headers["referer"] = pageURL
		headers["wm_page_url"] = pageURL
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func buildOrderPageURL(orderID, groupID string) string {
	query := url.Values{}
	query.Set("groupId", groupID)
	return fmt.Sprintf("https://www.walmart.com/orders/%s?%s", url.PathEscape(orderID), query.Encode())
}

func botChallengeError(body []byte) error {
	reference := strings.TrimSpace(string(body))
	if len(reference) > 128 {
		reference = reference[:128]
	}
	if reference == "" {
		return fmt.Errorf("%w: HTTP 456; refresh cookies and the request profile from a browser", ErrBotChallenge)
	}
	return fmt.Errorf("%w: HTTP 456 (reference %s); refresh cookies and the request profile from a browser", ErrBotChallenge, reference)
}

func randomUUID() string {
	b := randomBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}

func randomHex(byteCount int) string {
	return hex.EncodeToString(randomBytes(byteCount))
}

func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err == nil {
		return b
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	copy(b, sum[:])
	return b
}

// setCookies adds all cookies from the store to the request.
func (c *WalmartClient) setCookies(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allCookies := c.cookieStore.GetAll()
	cookiePairs := make([]string, 0, len(allCookies))
	for name, cookie := range allCookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", name, cookie.Value))
	}

	if len(cookiePairs) > 0 {
		req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
	}
}

// updateCookiesFromResponse updates cookie store with Set-Cookie headers.
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

				// Check if this is an update.
				existing := c.cookieStore.Get(name)
				if existing != nil && existing.Value != value {
					updatedCount++
				}

				c.cookieStore.Set(name, &cookies.Cookie{
					Value:      value,
					LastUpdate: time.Now(),
					Source:     "response",
					Essential:  existing != nil && existing.Essential,
				})
			}
		}
	}

	if updatedCount > 0 {
		c.logger.Debug("cookies updated from response",
			slog.Int("updated_count", updatedCount))
	}
}
