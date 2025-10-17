package walmart

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// PurchaseHistoryRequest represents the request parameters
type PurchaseHistoryRequest struct {
	Cursor       string   `json:"cursor"`       // Empty for first page
	Search       string   `json:"search"`       // Search filter (e.g., "cheese")
	FilterIds    []string `json:"filterIds"`    // Filter IDs (e.g., ["last-3-months", "in-store"])
	Limit        int      `json:"limit"`        // Number of orders to return
	Type         *string  `json:"type"`         // Order type (DELIVERY, PICKUP, etc.)
	MinTimestamp *int64   `json:"minTimestamp"` // Start date filter (unix timestamp)
	MaxTimestamp *int64   `json:"maxTimestamp"` // End date filter (unix timestamp)
}

// PurchaseHistoryResponse represents the response structure
type PurchaseHistoryResponse struct {
	Data struct {
		OrderHistoryV2 struct {
			PageInfo struct {
				NextPageCursor string `json:"nextPageCursor"`
				PrevPageCursor string `json:"prevPageCursor"`
			} `json:"pageInfo"`
			OrderGroups []OrderSummary `json:"orderGroups"`
		} `json:"orderHistoryV2"`
	} `json:"data"`
}

// OrderSummary represents a summary of an order in the history
type OrderSummary struct {
	Type                   string        `json:"type"` // IN_STORE, GLASS, etc.
	OrderID                string        `json:"orderId"`
	GroupID                string        `json:"groupId"`
	PurchaseOrderID        *string       `json:"purchaseOrderId"`
	FulfillmentType        string        `json:"fulfillmentType"`        // IN_STORE, DFS, etc.
	DerivedFulfillmentType string        `json:"derivedFulfillmentType"` // IN_STORE, SC_DELIVERY, etc.
	IsActive               bool          `json:"isActive"`
	ItemCount              int           `json:"itemCount"`
	DeliveryMessage        string        `json:"deliveryMessage"`
	Store                  *StoreInfo    `json:"store"`
	Status                 *StatusInfo   `json:"status"`
	Items                  []ItemSummary `json:"items"`
	DeliveredDate          *string       `json:"deliveredDate"`
}

// StoreInfo represents store information
type StoreInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address struct {
		AddressLineOne string `json:"addressLineOne"`
	} `json:"address"`
}

// StatusInfo represents order status
type StatusInfo struct {
	StatusType string `json:"statusType"` // IN_STORE, DELIVERED, etc.
	Message    struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"message"`
}

// ItemSummary represents an item in the order summary
type ItemSummary struct {
	ID        string `json:"id"`
	Quantity  int    `json:"quantity"`
	Name      string `json:"name"`
	ImageInfo struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"imageInfo"`
}

// GetPurchaseHistory fetches the purchase history with optional filters
func (c *WalmartClient) GetPurchaseHistory(req PurchaseHistoryRequest) (*PurchaseHistoryResponse, error) {
	// Rate limiting
	if !c.lastRequest.IsZero() {
		c.logger.Debug("waiting for rate limiter")
		<-c.rateLimiter.C
	}
	c.lastRequest = time.Now()

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 10
	}

	// Log the request with appropriate detail
	logAttrs := []any{slog.Int("limit", req.Limit)}
	if req.MinTimestamp != nil {
		logAttrs = append(logAttrs, slog.Int64("min_timestamp", *req.MinTimestamp))
	}
	if req.MaxTimestamp != nil {
		logAttrs = append(logAttrs, slog.Int64("max_timestamp", *req.MaxTimestamp))
	}
	if req.Search != "" {
		logAttrs = append(logAttrs, slog.String("search", req.Search))
	}
	c.logger.Info("fetching purchase history", logAttrs...)

	endpoint := c.buildPurchaseHistoryEndpoint(req)

	c.logger.Debug("purchase history request",
		slog.String("endpoint", endpoint))

	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		c.logger.Error("failed to create request",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (reuse existing method but adjust for purchase history)
	c.setPurchaseHistoryHeaders(httpReq)

	// Set cookies from store
	c.setCookies(httpReq)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("request failed",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.logger.Debug("received response",
		slog.Int("status_code", resp.StatusCode))

	// Update cookies from response
	c.updateCookiesFromResponse(resp)

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read response",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("response received",
		slog.Int("response_size", len(body)))

	// Check status
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limited",
				slog.Int("status_code", resp.StatusCode))
			return nil, fmt.Errorf("rate limited - cookies might be stale, try refreshing from browser")
		}
		if resp.StatusCode == 403 || resp.StatusCode == 418 {
			c.logger.Warn("access denied",
				slog.Int("status_code", resp.StatusCode))
			return nil, fmt.Errorf("access denied - cookies expired, please update from browser")
		}
		c.logger.Warn("non-200 response",
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var historyResp PurchaseHistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		c.logger.Error("failed to parse response",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.Info("fetched purchase history",
		slog.Int("order_count", len(historyResp.Data.OrderHistoryV2.OrderGroups)))

	// Auto-save cookies after successful request
	_ = c.CookieStore.Save()

	return &historyResp, nil
}

// GetRecentOrders is a convenience method to get recent orders
func (c *WalmartClient) GetRecentOrders(limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Limit: limit,
	}

	resp, err := c.GetPurchaseHistory(req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// GetAllOrders fetches all orders with pagination
func (c *WalmartClient) GetAllOrders(maxPages int) ([]OrderSummary, error) {
	var allOrders []OrderSummary
	cursor := ""

	for page := 0; page < maxPages; page++ {
		req := PurchaseHistoryRequest{
			Cursor: cursor,
			Limit:  20,
		}

		resp, err := c.GetPurchaseHistory(req)
		if err != nil {
			return allOrders, fmt.Errorf("failed on page %d: %w", page+1, err)
		}

		allOrders = append(allOrders, resp.Data.OrderHistoryV2.OrderGroups...)

		// Check if there's a next page
		cursor = resp.Data.OrderHistoryV2.PageInfo.NextPageCursor
		if cursor == "" {
			break
		}

		fmt.Printf("Fetched page %d, got %d orders (total: %d)\n",
			page+1, len(resp.Data.OrderHistoryV2.OrderGroups), len(allOrders))
	}

	return allOrders, nil
}

// SearchOrders searches for orders containing a specific item
func (c *WalmartClient) SearchOrders(searchTerm string, limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Search: searchTerm,
		Limit:  limit,
	}

	resp, err := c.GetPurchaseHistory(req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// GetOrdersByType fetches orders of a specific type
func (c *WalmartClient) GetOrdersByType(orderType string, limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Type:  &orderType,
		Limit: limit,
	}

	resp, err := c.GetPurchaseHistory(req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// Helper to build the purchase history endpoint
func (c *WalmartClient) buildPurchaseHistoryEndpoint(req PurchaseHistoryRequest) string {
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"cursor":       req.Cursor,
			"search":       req.Search,
			"filterIds":    req.FilterIds,
			"limit":        req.Limit,
			"type":         req.Type,
			"minTimestamp": req.MinTimestamp,
			"maxTimestamp": req.MaxTimestamp,
		},
		"platform": "WEB",
	}

	variablesJSON, _ := json.Marshal(variables)
	params := url.Values{}
	params.Set("variables", string(variablesJSON))

	// Different hash for PurchaseHistoryV2
	return fmt.Sprintf("https://www.walmart.com/orchestra/cph/graphql/PurchaseHistoryV2/2c3d5a832b56671dca1ed0ec84940f274d0bc80821db4ad7481e496c0ad5847e?%s",
		params.Encode())
}

// Set headers specific to purchase history
func (c *WalmartClient) setPurchaseHistoryHeaders(req *http.Request) {
	headers := map[string]string{
		"accept":                  "application/json",
		"accept-language":         "en-US",
		"content-type":            "application/json",
		"user-agent":              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
		"x-apollo-operation-name": "PurchaseHistoryV2",
		"x-o-gql-query":           "query PurchaseHistoryV2",
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
