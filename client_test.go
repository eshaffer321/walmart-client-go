package walmart

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWalmartClient(t *testing.T) {
	tempDir := t.TempDir()
	config := ClientConfig{
		CookieDir: tempDir,
		RateLimit: 1 * time.Second,
		AutoSave:  true,
	}

	client, err := NewWalmartClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	if client.CookieStore == nil {
		t.Fatal("CookieStore is nil")
	}

	expectedPath := filepath.Join(tempDir, "cookies.json")
	if client.CookieStore.FilePath != expectedPath {
		t.Errorf("Expected cookie path %s, got %s", expectedPath, client.CookieStore.FilePath)
	}
}

func TestCookieStore(t *testing.T) {
	tempDir := t.TempDir()
	store := &CookieStore{
		Cookies:  make(map[string]*Cookie),
		FilePath: filepath.Join(tempDir, "test_cookies.json"),
	}

	// Test Set and Get
	cookie := &Cookie{
		Value:      "test_value",
		LastUpdate: time.Now(),
		Source:     "test",
		Essential:  true,
	}

	store.Set("test_cookie", cookie)
	retrieved := store.Get("test_cookie")

	if retrieved == nil {
		t.Fatal("Failed to retrieve cookie")
	}

	if retrieved.Value != "test_value" {
		t.Errorf("Expected value 'test_value', got '%s'", retrieved.Value)
	}

	// Test Save and Load
	err := store.Save()
	if err != nil {
		t.Fatalf("Failed to save cookies: %v", err)
	}

	newStore := &CookieStore{
		Cookies:  make(map[string]*Cookie),
		FilePath: store.FilePath,
	}

	err = newStore.Load()
	if err != nil {
		t.Fatalf("Failed to load cookies: %v", err)
	}

	loaded := newStore.Get("test_cookie")
	if loaded == nil {
		t.Fatal("Failed to load cookie from file")
	}

	if loaded.Value != "test_value" {
		t.Errorf("Loaded cookie has wrong value: %s", loaded.Value)
	}
}

func TestExtractCookiesFromCurl(t *testing.T) {
	curlCommand := `curl 'https://example.com' \
  -b 'cookie1=value1; cookie2=value2' \
  --cookie 'cookie3=value3'`

	cookies := extractCookiesFromCurl(curlCommand)

	expected := map[string]string{
		"cookie1": "value1",
		"cookie2": "value2",
		"cookie3": "value3",
	}

	for name, expectedValue := range expected {
		if value, ok := cookies[name]; !ok {
			t.Errorf("Cookie %s not found", name)
		} else if value != expectedValue {
			t.Errorf("Cookie %s: expected %s, got %s", name, expectedValue, value)
		}
	}
}

func TestOrderModels(t *testing.T) {
	order := &Order{
		ID:        "123",
		DisplayID: "WM-123",
		Groups: []OrderGroup{
			{
				Items: []OrderItem{
					{
						Quantity: 2.0,
						PriceInfo: &ItemPrice{
							LinePrice: &Price{Value: 10.00},
						},
					},
					{
						Quantity: 1.0,
						PriceInfo: &ItemPrice{
							LinePrice: &Price{Value: 5.00},
						},
					},
				},
				ItemCount: 3,
			},
		},
	}

	// Test GetItems
	items := order.GetItems()
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	// Test GetItemCount
	count := order.GetItemCount()
	if count != 3 {
		t.Errorf("Expected item count 3, got %d", count)
	}

	// Test CalculateOrderTotal
	total := order.CalculateOrderTotal()
	if total != 15.00 {
		t.Errorf("Expected total 15.00, got %.2f", total)
	}
}

func TestUpdateCookiesFromResponse(t *testing.T) {
	tempDir := t.TempDir()
	config := ClientConfig{
		CookieDir: tempDir,
	}

	client, _ := NewWalmartClient(config)

	// Add initial cookie
	client.CookieStore.Set("existing", &Cookie{
		Value:     "old_value",
		Essential: true,
	})

	// Create mock response with Set-Cookie headers
	resp := &http.Response{
		Header: http.Header{
			"Set-Cookie": []string{
				"existing=new_value; Path=/",
				"new_cookie=value; HttpOnly",
			},
		},
	}

	client.updateCookiesFromResponse(resp)

	// Check existing cookie was updated
	existing := client.CookieStore.Get("existing")
	if existing == nil || existing.Value != "new_value" {
		t.Error("Failed to update existing cookie")
	}
	if !existing.Essential {
		t.Error("Lost essential flag on update")
	}

	// Check new cookie was added
	newCookie := client.CookieStore.Get("new_cookie")
	if newCookie == nil || newCookie.Value != "value" {
		t.Error("Failed to add new cookie from response")
	}
}

func TestBuildOrderEndpoint(t *testing.T) {
	client, _ := NewWalmartClient(ClientConfig{})

	endpoint := client.buildOrderEndpoint("TEST123", true)

	if endpoint == "" {
		t.Error("Endpoint is empty")
	}

	// Check it contains the order ID
	if !contains(endpoint, "TEST123") {
		t.Error("Endpoint doesn't contain order ID")
	}

	// Check it has the GraphQL hash
	if !contains(endpoint, "d0622497daef19150438d07c506739d451cad6749cf45c3b4db95f2f5a0a65c4") {
		t.Error("Endpoint doesn't contain correct GraphQL hash")
	}
}

func TestMockOrderRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-apollo-operation-name") != "getOrder" {
			t.Error("Missing or wrong Apollo operation header")
		}

		// Send mock response
		response := OrderResponse{
			Data: struct {
				Order *Order `json:"order"`
			}{
				Order: &Order{
					ID:        "TEST123",
					DisplayID: "WM-TEST123",
					OrderDate: "2024-01-01T12:00:00.000-0700",
					PriceDetails: &OrderPriceDetails{
						GrandTotal: &PriceLineItem{
							DisplayValue: "$100.00",
							Value:        100.00,
						},
					},
				},
			},
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test server
	client, _ := NewWalmartClient(ClientConfig{})
	client.httpClient = server.Client()

	// Test with mock server URL - we can't override the method directly
	// so we'll just test that the request would be made correctly

	// Add required cookies
	client.CookieStore.Set("CID", &Cookie{Value: "test"})
	client.CookieStore.Set("SPID", &Cookie{Value: "test"})

	// Since we can't override the endpoint builder, just verify the endpoint is built correctly
	endpoint := client.buildOrderEndpoint("TEST123", true)
	if !contains(endpoint, "TEST123") {
		t.Error("Endpoint doesn't contain order ID")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestInitializeFromCurl(t *testing.T) {
	tempDir := t.TempDir()

	// Create test curl file
	curlContent := `curl 'https://www.walmart.com/test' \
  -b 'CID=test_cid; SPID=test_spid; auth=test_auth'`

	curlFile := filepath.Join(tempDir, "test_curl.txt")
	_ = os.WriteFile(curlFile, []byte(curlContent), 0644)

	config := ClientConfig{
		CookieDir: tempDir,
	}

	client, _ := NewWalmartClient(config)
	err := client.InitializeFromCurl(curlFile)

	if err != nil {
		t.Fatalf("Failed to initialize from curl: %v", err)
	}

	// Check essential cookies were loaded
	cid := client.CookieStore.Get("CID")
	if cid == nil || cid.Value != "test_cid" {
		t.Error("CID cookie not loaded correctly")
	}
	if !cid.Essential {
		t.Error("CID should be marked as essential")
	}

	spid := client.CookieStore.Get("SPID")
	if spid == nil || spid.Value != "test_spid" {
		t.Error("SPID cookie not loaded correctly")
	}
	if !spid.Essential {
		t.Error("SPID should be marked as essential")
	}
}

func TestParseOrderWithDecimalQuantities(t *testing.T) {
	// This is actual JSON from a Walmart order with weighted produce
	jsonData := `{
		"data": {
			"order": {
				"id": "200013427048402",
				"groups_2101": [{
					"items": [{
						"id": "1",
						"quantity": 1.081,
						"productInfo": {
							"name": "Bananas, sold by weight",
							"usItemId": "44390948"
						},
						"priceInfo": {
							"linePrice": {
								"value": 0.58
							}
						}
					}, {
						"id": "2",
						"quantity": 0.299,
						"productInfo": {
							"name": "Roma Tomatoes",
							"usItemId": "44391210"
						},
						"priceInfo": {
							"linePrice": {
								"value": 0.44
							}
						}
					}]
				}]
			}
		}
	}`

	var response OrderResponse
	err := json.Unmarshal([]byte(jsonData), &response)

	// This test will FAIL with current implementation
	if err != nil {
		t.Fatalf("Should handle decimal quantities, but got error: %v", err)
	}

	if len(response.Data.Order.Groups) == 0 {
		t.Fatal("No groups found in response")
	}

	if len(response.Data.Order.Groups[0].Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(response.Data.Order.Groups[0].Items))
	}

	// Check the decimal quantities parsed correctly
	if response.Data.Order.Groups[0].Items[0].Quantity != 1.081 {
		t.Errorf("Expected quantity 1.081, got %v", response.Data.Order.Groups[0].Items[0].Quantity)
	}

	if response.Data.Order.Groups[0].Items[1].Quantity != 0.299 {
		t.Errorf("Expected quantity 0.299, got %v", response.Data.Order.Groups[0].Items[1].Quantity)
	}
}

func TestParseOrderWithWholeNumberQuantities(t *testing.T) {
	// Test that whole numbers still work (quantity: 1 not 1.0 in JSON)
	jsonData := `{
		"data": {
			"order": {
				"groups_2101": [{
					"items": [{
						"quantity": 2,
						"productInfo": {"name": "Milk"}
					}]
				}]
			}
		}
	}`

	var response OrderResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	
	if err != nil {
		t.Fatalf("Should handle whole number quantities, but got error: %v", err)
	}

	if len(response.Data.Order.Groups) == 0 || len(response.Data.Order.Groups[0].Items) == 0 {
		t.Fatal("No items found in response")
	}

	// After fixing to float64, this should be 2.0
	expectedQuantity := 2.0
	if response.Data.Order.Groups[0].Items[0].Quantity != expectedQuantity {
		t.Errorf("Expected quantity %v, got %v", expectedQuantity, response.Data.Order.Groups[0].Items[0].Quantity)
	}
}
