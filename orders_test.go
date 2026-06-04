package walmart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockClient builds a client whose HTTP traffic is redirected to serverURL
// and whose cookies are stored under a temp dir so tests stay hermetic.
func newMockClient(t *testing.T, serverURL string) *WalmartClient {
	t.Helper()
	client, err := NewWalmartClient(ClientConfig{
		RateLimit:  time.Millisecond,
		AutoSave:   false,
		CookieFile: filepath.Join(t.TempDir(), "cookies.json"),
	})
	require.NoError(t, err)
	client.httpClient = &http.Client{Transport: &testTransport{serverURL: serverURL}}
	return client
}

const inStoreOrderJSON = `{
	"data": {
		"order": {
			"id": "200013509224581",
			"displayId": "D-12345",
			"orderDate": "2025-01-02T10:00:00.000-0700",
			"groups_2101": [{
				"itemCount": 2,
				"fulfillmentType": "IN_STORE",
				"items": [
					{"id": "i1", "quantity": 1, "productInfo": {"name": "Milk", "usItemId": "u1"}, "priceInfo": {"linePrice": {"displayValue": "$3.00", "value": 3.0}}},
					{"id": "i2", "quantity": 1, "productInfo": {"name": "Bread", "usItemId": "u2"}, "priceInfo": {"linePrice": {"displayValue": "$2.50", "value": 2.5}}}
				]
			}],
			"priceDetails": {"grandTotal": {"displayValue": "$10.00", "value": 10.0}}
		}
	}
}`

func TestGetOrderSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/orchestra/orders/graphql/getOrder/")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(inStoreOrderJSON))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	order, err := client.GetOrder(context.Background(), "200013509224581", true)

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, "200013509224581", order.ID)
	assert.Equal(t, "D-12345", order.DisplayID)
	assert.Equal(t, 2, order.GetItemCount())
	require.NotNil(t, order.PriceDetails.GrandTotal)
	assert.Equal(t, 10.0, order.PriceDetails.GrandTotal.Value)
}

func TestGetOrderDeliveryCalculatesTotalWithTip(t *testing.T) {
	const deliveryJSON = `{
		"data": {
			"order": {
				"id": "delivery-1",
				"groups_2101": [{"itemCount": 1, "fulfillmentType": "SC_DELIVERY", "items": []}],
				"priceDetails": {
					"grandTotal": {"displayValue": "$20.00", "value": 20.0},
					"driverTip": {"displayValue": "$5.00", "value": 5.0}
				}
			}
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(deliveryJSON))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	order, err := client.GetOrder(context.Background(), "delivery-1", false)

	require.NoError(t, err)
	require.NotNil(t, order.PriceDetails.TotalWithTip)
	assert.Equal(t, 25.0, order.PriceDetails.TotalWithTip.Value)
}

func TestGetOrderStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErrMsg string
	}{
		{"rate limited", http.StatusTooManyRequests, "rate limited"},
		{"forbidden", http.StatusForbidden, "access denied"},
		{"teapot", http.StatusTeapot, "access denied"},
		{"server error", http.StatusInternalServerError, "HTTP 500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error": "nope"}`))
			}))
			defer server.Close()

			client := newMockClient(t, server.URL)
			order, err := client.GetOrder(context.Background(), "x", true)

			require.Error(t, err)
			assert.Nil(t, order)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestGetOrderMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	_, err := client.GetOrder(context.Background(), "x", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestGetOrderNoOrderData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"order": null}}`))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	_, err := client.GetOrder(context.Background(), "x", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no order data in response")
}

func TestGetOrderAutoDetectSucceedsOnFirstTry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(inStoreOrderJSON))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	order, err := client.GetOrderAutoDetect(context.Background(), "200013509224581")

	require.NoError(t, err)
	assert.Equal(t, "200013509224581", order.ID)
}

func TestGetOrderAutoDetectFailsBothModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	_, err := client.GetOrderAutoDetect(context.Background(), "x")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "as either in-store or delivery")
}

func TestGetOrderAsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(inStoreOrderJSON))
	}))
	defer server.Close()

	client := newMockClient(t, server.URL)
	out, err := client.GetOrderAsJSON(context.Background(), "200013509224581", true)
	require.NoError(t, err)

	// Output must be valid, re-parseable JSON for the order.
	var parsed Order
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	assert.Equal(t, "200013509224581", parsed.ID)
}
