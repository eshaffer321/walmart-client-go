package walmart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// PurchaseHistoryRequest represents the request parameters.
type PurchaseHistoryRequest struct {
	Cursor       string   `json:"cursor"`       // Empty for first page.
	Search       string   `json:"search"`       // Search filter (e.g., "cheese").
	FilterIds    []string `json:"filterIds"`    // Filter IDs (e.g., ["last-3-months", "in-store"]).
	Limit        int      `json:"limit"`        // Number of orders to return.
	Type         *string  `json:"type"`         // Order type (DELIVERY, PICKUP, etc.)
	MinTimestamp *int64   `json:"minTimestamp"` // Start date filter (unix timestamp).
	MaxTimestamp *int64   `json:"maxTimestamp"` // End date filter (unix timestamp).
}

// PurchaseHistoryResponse represents the response structure.
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

// OrderSummary represents a summary of an order in the history.
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

// StoreInfo represents store information.
type StoreInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address struct {
		AddressLineOne string `json:"addressLineOne"`
	} `json:"address"`
}

// StatusInfo represents order status.
type StatusInfo struct {
	StatusType string `json:"statusType"` // IN_STORE, DELIVERED, etc.
	Message    struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"message"`
}

// ItemSummary represents an item in the order summary.
type ItemSummary struct {
	ID        string `json:"id"`
	Quantity  int    `json:"quantity"`
	Name      string `json:"name"`
	ImageInfo struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"imageInfo"`
}

// GetPurchaseHistory fetches the purchase history with optional filters.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetPurchaseHistory(ctx context.Context, req PurchaseHistoryRequest) (*PurchaseHistoryResponse, error) {
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

	// Set defaults.
	if req.Limit == 0 {
		req.Limit = 10
	}

	// Log the request with appropriate detail.
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

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		c.logger.Error("failed to create request",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (reuse existing method but adjust for purchase history).
	c.setPurchaseHistoryHeaders(httpReq)

	// Set cookies from store.
	c.setCookies(httpReq)

	// Execute request.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("request failed",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.logger.Debug("failed to close response body", slog.String("error", cerr.Error()))
		}
	}()

	c.logger.Debug("received response",
		slog.Int("status_code", resp.StatusCode))

	// Update cookies from response.
	c.updateCookiesFromResponse(resp)

	// Read body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read response",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("response received",
		slog.Int("response_size", len(body)))

	// Check status.
	if err := c.checkPurchaseResponseError(resp, body); err != nil {
		return nil, err
	}

	// Parse response.
	var historyResp PurchaseHistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		c.logger.Error("failed to parse response",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.Info("fetched purchase history",
		slog.Int("order_count", len(historyResp.Data.OrderHistoryV2.OrderGroups)))

	// Auto-save cookies after successful request.
	if err := c.cookieStore.Save(); err != nil {
		c.logger.Warn("failed to save cookies after request", slog.String("error", err.Error()))
	}

	return &historyResp, nil
}

// checkPurchaseResponseError returns an error describing a non-OK purchase
// history response, or nil when the status is HTTP 200.
func (c *WalmartClient) checkPurchaseResponseError(resp *http.Response, body []byte) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	switch resp.StatusCode {
	case 429:
		c.logger.Warn("rate limited", slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("rate limited - cookies might be stale, try refreshing from browser")
	case 403, 418:
		c.logger.Warn("access denied", slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("access denied - cookies expired, please update from browser")
	case 456:
		c.logger.Warn("bot challenge", slog.Int("status_code", resp.StatusCode))
		return botChallengeError(body)
	default:
		c.logger.Warn("non-200 response", slog.Int("status_code", resp.StatusCode))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
}

// GetRecentOrders is a convenience method to get recent orders.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetRecentOrders(ctx context.Context, limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Limit: limit,
	}

	resp, err := c.GetPurchaseHistory(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// GetAllOrders fetches all orders with pagination.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetAllOrders(ctx context.Context, maxPages int) ([]OrderSummary, error) {
	var allOrders []OrderSummary
	cursor := ""

	for page := 0; page < maxPages; page++ {
		// Check for cancellation before each page.
		if err := ctx.Err(); err != nil {
			return allOrders, fmt.Errorf("canceled after %d pages: %w", page, err)
		}

		req := PurchaseHistoryRequest{
			Cursor: cursor,
			Limit:  20,
		}

		resp, err := c.GetPurchaseHistory(ctx, req)
		if err != nil {
			return allOrders, fmt.Errorf("failed on page %d: %w", page+1, err)
		}

		allOrders = append(allOrders, resp.Data.OrderHistoryV2.OrderGroups...)

		// Check if there's a next page.
		cursor = resp.Data.OrderHistoryV2.PageInfo.NextPageCursor
		if cursor == "" {
			break
		}

		c.logger.Debug("fetched page",
			slog.Int("page", page+1),
			slog.Int("page_orders", len(resp.Data.OrderHistoryV2.OrderGroups)),
			slog.Int("total_orders", len(allOrders)))
	}

	return allOrders, nil
}

// SearchOrders searches for orders containing a specific item.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) SearchOrders(ctx context.Context, searchTerm string, limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Search: searchTerm,
		Limit:  limit,
	}

	resp, err := c.GetPurchaseHistory(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// GetOrdersByType fetches orders of a specific type.
// The context can be used to cancel the request or set a deadline.
func (c *WalmartClient) GetOrdersByType(ctx context.Context, orderType string, limit int) ([]OrderSummary, error) {
	req := PurchaseHistoryRequest{
		Type:  &orderType,
		Limit: limit,
	}

	resp, err := c.GetPurchaseHistory(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Data.OrderHistoryV2.OrderGroups, nil
}

// Helper to build the purchase history endpoint.
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

	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		c.logger.Error("failed to marshal purchase history variables", slog.String("error", err.Error()))
	}
	params := url.Values{}
	params.Set("variables", string(variablesJSON))

	// Different hash for PurchaseHistoryV2.
	return fmt.Sprintf("https://www.walmart.com/orchestra/cph/graphql/PurchaseHistoryV2/2c3d5a832b56671dca1ed0ec84940f274d0bc80821db4ad7481e496c0ad5847e?%s",
		params.Encode())
}

// Set headers specific to purchase history.
func (c *WalmartClient) setPurchaseHistoryHeaders(req *http.Request) {
	c.setHeaders(req, "PurchaseHistoryV2", "https://www.walmart.com/orders")
}
