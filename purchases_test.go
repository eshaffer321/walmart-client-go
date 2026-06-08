package walmart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rateLimitedMsg  = "rate limited"
	accessDeniedMsg = "access denied"
)

// purchaseHistoryJSON builds a single-page response with the given cursor and
// order count. An empty cursor signals the last page.
func purchaseHistoryJSON(nextCursor string, orderIDs ...string) string {
	groups := ""
	for i, id := range orderIDs {
		if i > 0 {
			groups += ","
		}
		groups += fmt.Sprintf(`{"orderId": %q, "fulfillmentType": "IN_STORE", "itemCount": 1}`, id)
	}
	return fmt.Sprintf(`{
		"data": {
			"orderHistoryV2": {
				"pageInfo": {"nextPageCursor": %q, "prevPageCursor": ""},
				"orderGroups": [%s]
			}
		}
	}`, nextCursor, groups)
}

func TestGetPurchaseHistorySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/orchestra/cph/graphql/PurchaseHistoryV2/")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(purchaseHistoryJSON("", "order-1", "order-2")))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	resp, err := client.GetPurchaseHistory(context.Background(), PurchaseHistoryRequest{Limit: 10})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Data.OrderHistoryV2.OrderGroups, 2)
	assert.Equal(t, "order-1", resp.Data.OrderHistoryV2.OrderGroups[0].OrderID)
}

func TestGetPurchaseHistoryStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErrMsg string
	}{
		{rateLimitedMsg, http.StatusTooManyRequests, rateLimitedMsg},
		{"forbidden", http.StatusForbidden, accessDeniedMsg},
		{"teapot", http.StatusTeapot, accessDeniedMsg},
		{"server error", http.StatusInternalServerError, "HTTP 500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := newMockClient(t, server.URL)
			_, err := client.GetPurchaseHistory(context.Background(), PurchaseHistoryRequest{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestGetPurchaseHistoryMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{nope`))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	_, err := client.GetPurchaseHistory(context.Background(), PurchaseHistoryRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestGetRecentOrders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(purchaseHistoryJSON("", "recent-1")))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	orders, err := client.GetRecentOrders(context.Background(), 5)

	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, "recent-1", orders[0].OrderID)
}

func TestGetAllOrdersPaginates(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			// First page points to a second page.
			_, _ = w.Write([]byte(purchaseHistoryJSON("cursor-page-2", "p1-order")))
		default:
			// Second page ends pagination with an empty cursor.
			_, _ = w.Write([]byte(purchaseHistoryJSON("", "p2-order")))
		}
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	orders, err := client.GetAllOrders(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 2, calls, "should stop once the cursor is empty")
	require.Len(t, orders, 2)
	assert.Equal(t, "p1-order", orders[0].OrderID)
	assert.Equal(t, "p2-order", orders[1].OrderID)
}

func TestSearchOrdersAndGetOrdersByType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(purchaseHistoryJSON("", "match-1")))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)

	searched, err := client.SearchOrders(context.Background(), "cheese", 10)
	require.NoError(t, err)
	require.Len(t, searched, 1)
	assert.Equal(t, "match-1", searched[0].OrderID)

	byType, err := client.GetOrdersByType(context.Background(), "DELIVERY", 10)
	require.NoError(t, err)
	require.Len(t, byType, 1)
}

func TestGetOrdersAsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(purchaseHistoryJSON("", "json-order")))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	out, err := client.GetOrdersAsJSON(context.Background(), 5)
	require.NoError(t, err)

	var parsed []OrderSummary
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	require.Len(t, parsed, 1)
	assert.Equal(t, "json-order", parsed[0].OrderID)
}
