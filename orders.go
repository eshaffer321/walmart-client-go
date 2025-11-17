package walmart

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/eshaffer321/walmart-client-go/internal/cookies"
)

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
	c.setHeaders(req, "getOrder")

	// Execute request (cookies are handled automatically by AuthenticatedTransport)
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
	_ = c.cookieStore.Save()

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

// GetOrdersAsJSON is a helper that returns recent orders as JSON string
func (c *WalmartClient) GetOrdersAsJSON(limit int) (string, error) {
	orders, err := c.GetRecentOrders(limit)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return string(jsonData), nil
}

// GetOrderAsJSON is a helper that returns order details as JSON string
func (c *WalmartClient) GetOrderAsJSON(orderID string, isInStore bool) (string, error) {
	order, err := c.GetOrder(orderID, isInStore)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return string(jsonData), nil
}

// buildOrderEndpoint constructs the GraphQL endpoint URL for fetching order details
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

// setHeaders sets standard HTTP headers required by Walmart's API
func (c *WalmartClient) setHeaders(req *http.Request, operationName string) {
	headers := map[string]string{
		"accept":                  "application/json",
		"accept-language":         "en-US",
		"content-type":            "application/json",
		"user-agent":              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
		"x-apollo-operation-name": operationName,
		"x-o-gql-query":           fmt.Sprintf("query %s", operationName),
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

// setCookies adds all cookies from the store to the request
//
// Deprecated: Cookie management is now handled automatically by the
// AuthenticatedTransport layer. This method is kept for backward compatibility
// but is no longer necessary. It will be removed in v2.0.0.
func (c *WalmartClient) setCookies(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	allCookies := c.cookieStore.GetAll()
	var cookiePairs []string
	for name, cookie := range allCookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", name, cookie.Value))
	}

	if len(cookiePairs) > 0 {
		req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
	}
}

// updateCookiesFromResponse updates cookie store with Set-Cookie headers
//
// Deprecated: Cookie management is now handled automatically by the
// AuthenticatedTransport layer. This method is kept for backward compatibility
// but is no longer necessary. It will be removed in v2.0.0.
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
