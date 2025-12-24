package walmart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTransport redirects requests to our test server
type testTransport struct {
	serverURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the host with our test server
	testURL, _ := url.Parse(t.serverURL)
	req.URL.Host = testURL.Host
	req.URL.Scheme = testURL.Scheme

	// Use default transport for actual request
	return http.DefaultTransport.RoundTrip(req)
}

func TestGetOrderLedger(t *testing.T) {
	tests := []struct {
		name           string
		orderID        string
		mockResponse   string
		expectedLedger *OrderLedger
		wantErr        bool
		errMessage     string
	}{
		{
			name:    "successful ledger retrieval with credit card and walmart cash",
			orderID: "200013509224581",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": [
							{
								"paymentType": "CREDITCARD",
								"cardType": "VISA",
								"description": "Ending in 0953",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "December 16, 2024",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$178.96"],
														"time": "4:31 AM"
													}
												]
											},
											{
												"date": "December 17, 2024",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$4.12"],
														"time": "7:31 PM"
													}
												]
											}
										]
									},
									{
										"chargeType": "TEMPORARY_HOLDS",
										"transactionLines": [
											{
												"date": "December 16, 2024",
												"rowLines": [
													{
														"lineType": "ORDER_AUTHORIZATION",
														"displayValues": ["$185.83"],
														"time": "3:41 AM"
													}
												]
											}
										]
									}
								]
							},
							{
								"paymentType": "GIFTCARD",
								"cardType": "WMTRC",
								"description": "Walmart Cash",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "December 16, 2024",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$2.75"],
														"time": "4:31 AM"
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID: "200013509224581",
				PaymentMethods: []PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "VISA",
						LastFour:     "0953",
						FinalCharges: []float64{178.96, 4.12},
						ChargedDates: []time.Time{
							time.Date(2024, time.December, 16, 4, 31, 0, 0, time.UTC),
							time.Date(2024, time.December, 17, 19, 31, 0, 0, time.UTC),
						},
						TotalCharged: 183.08,
					},
					{
						PaymentType:  "GIFTCARD",
						CardType:     "WMTRC",
						LastFour:     "",
						FinalCharges: []float64{2.75},
						ChargedDates: []time.Time{
							time.Date(2024, time.December, 16, 4, 31, 0, 0, time.UTC),
						},
						TotalCharged: 2.75,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "ledger with order adjustments and refunds",
			orderID: "200013509224582",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": [
							{
								"paymentType": "CREDITCARD",
								"cardType": "MASTERCARD",
								"description": "Ending in 1234",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "January 10, 2025",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$100.00"],
														"time": "10:00 AM"
													},
													{
														"lineType": "ORDER_ADJUSTMENT_REFUND",
														"displayValues": ["-$15.00"],
														"time": "11:00 AM"
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID: "200013509224582",
				PaymentMethods: []PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "MASTERCARD",
						LastFour:     "1234",
						FinalCharges: []float64{100.00, -15.00},
						ChargedDates: []time.Time{
							time.Date(2025, time.January, 10, 10, 0, 0, 0, time.UTC),
							time.Date(2025, time.January, 10, 11, 0, 0, 0, time.UTC),
						},
						TotalCharged: 85.00,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "single credit card payment",
			orderID: "200013509224583",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": [
							{
								"paymentType": "CREDITCARD",
								"cardType": "AMEX",
								"description": "Ending in 5678",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "January 15, 2025",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$250.50"],
														"time": "2:30 PM"
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID: "200013509224583",
				PaymentMethods: []PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "AMEX",
						LastFour:     "5678",
						FinalCharges: []float64{250.50},
						ChargedDates: []time.Time{
							time.Date(2025, time.January, 15, 14, 30, 0, 0, time.UTC),
						},
						TotalCharged: 250.50,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty ledger response",
			orderID: "200013509224584",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": []
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID:        "200013509224584",
				PaymentMethods: []PaymentMethodCharges{},
			},
			wantErr: false,
		},
		{
			name:       "invalid order ID",
			orderID:    "",
			wantErr:    true,
			errMessage: "order ID is required",
		},
		{
			name:         "malformed JSON response",
			orderID:      "200013509224585",
			mockResponse: `{invalid json}`,
			wantErr:      true,
		},
		{
			name:    "payment method without description",
			orderID: "200013509224586",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": [
							{
								"paymentType": "CREDITCARD",
								"cardType": "DISCOVER",
								"description": "",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "January 20, 2025",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$75.00"],
														"time": "5:00 PM"
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID: "200013509224586",
				PaymentMethods: []PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "DISCOVER",
						LastFour:     "",
						FinalCharges: []float64{75.00},
						ChargedDates: []time.Time{
							time.Date(2025, time.January, 20, 17, 0, 0, 0, time.UTC),
						},
						TotalCharged: 75.00,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "multiple display values in single row line",
			orderID: "200013509224587",
			mockResponse: `{
				"data": {
					"getOrderLedger": {
						"paymentMethodsLedgers": [
							{
								"paymentType": "CREDITCARD",
								"cardType": "VISA",
								"description": "Ending in 9999",
								"transactions": [
									{
										"chargeType": "FINAL_CHARGES",
										"transactionLines": [
											{
												"date": "January 25, 2025",
												"rowLines": [
													{
														"lineType": "ORDER_CHARGE",
														"displayValues": ["$50.00", "$25.00"],
														"time": "6:00 PM"
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}`,
			expectedLedger: &OrderLedger{
				OrderID: "200013509224587",
				PaymentMethods: []PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "VISA",
						LastFour:     "9999",
						FinalCharges: []float64{50.00, 25.00},
						ChargedDates: []time.Time{
							time.Date(2025, time.January, 25, 18, 0, 0, 0, time.UTC),
							time.Date(2025, time.January, 25, 18, 0, 0, 0, time.UTC),
						},
						TotalCharged: 75.00,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/orchestra/orders/graphql/getOrderLedger/")

				if tt.orderID != "" {
					assert.Contains(t, r.URL.Query().Get("variables"), tt.orderID)
				}

				if tt.mockResponse != "" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(tt.mockResponse))
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			config := ClientConfig{
				RateLimit: time.Millisecond * 100,
				AutoSave:  false,
			}
			client, err := NewWalmartClient(config)
			require.NoError(t, err)

			// Override the httpClient to use our test server
			client.httpClient = &http.Client{
				Transport: &testTransport{
					serverURL: server.URL,
				},
			}

			ctx := context.Background()
			ledger, err := client.GetOrderLedger(ctx, tt.orderID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedLedger, ledger)
			}
		})
	}
}

// TestGetOrderLedgerRateLimiting verifies rate limiting behavior
func TestGetOrderLedgerRateLimiting(t *testing.T) {
	requestTimes := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"getOrderLedger": {
					"paymentMethodsLedgers": []
				}
			}
		}`))
	}))
	defer server.Close()

	config := ClientConfig{
		RateLimit: 100 * time.Millisecond, // 100ms between requests
		AutoSave:  false,
	}
	client, err := NewWalmartClient(config)
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: &testTransport{
			serverURL: server.URL,
		},
	}

	// Make 3 ledger requests
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := client.GetOrderLedger(ctx, "test-order")
		require.NoError(t, err)
	}

	// Verify timing: first request immediate, subsequent requests delayed
	require.Len(t, requestTimes, 3)

	// First request should be immediate (no delay)
	// Second request should be ~100ms after first
	timeDiff1 := requestTimes[1].Sub(requestTimes[0])
	assert.GreaterOrEqual(t, timeDiff1.Milliseconds(), int64(90), "Second request should wait ~100ms")

	// Third request should be ~100ms after second
	timeDiff2 := requestTimes[2].Sub(requestTimes[1])
	assert.GreaterOrEqual(t, timeDiff2.Milliseconds(), int64(90), "Third request should wait ~100ms")
}

// TestGetOrderLedger429Response verifies 429 error handling
func TestGetOrderLedger429Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	config := ClientConfig{
		RateLimit:  time.Millisecond * 10,
		AutoSave:   false,
		MaxRetries: -1, // Disable retries for this test
	}
	client, err := NewWalmartClient(config)
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: &testTransport{
			serverURL: server.URL,
		},
	}

	ctx := context.Background()
	ledger, err := client.GetOrderLedger(ctx, "test-order")
	require.Error(t, err)
	assert.Nil(t, ledger)
	assert.Contains(t, err.Error(), "rate limited")
}

// TestGetOrderLedger403Response verifies 403/418 error handling
func TestGetOrderLedger403Response(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"forbidden 403", http.StatusForbidden},
		{"teapot 418", http.StatusTeapot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error": "access denied"}`))
			}))
			defer server.Close()

			config := ClientConfig{
				RateLimit: time.Millisecond * 10,
				AutoSave:  false,
			}
			client, err := NewWalmartClient(config)
			require.NoError(t, err)

			client.httpClient = &http.Client{
				Transport: &testTransport{
					serverURL: server.URL,
				},
			}

			ctx := context.Background()
			ledger, err := client.GetOrderLedger(ctx, "test-order")
			require.Error(t, err)
			assert.Nil(t, ledger)
			assert.Contains(t, err.Error(), "access denied")
			assert.Contains(t, err.Error(), "cookies might be stale")
		})
	}
}

// TestGetOrderLedgerContextCancellation verifies context cancellation works
func TestGetOrderLedgerContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"getOrderLedger": {"paymentMethodsLedgers": []}}}`))
	}))
	defer server.Close()

	config := ClientConfig{
		RateLimit: time.Millisecond * 10,
		AutoSave:  false,
	}
	client, err := NewWalmartClient(config)
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: &testTransport{
			serverURL: server.URL,
		},
	}

	// Create a context that we cancel immediately
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ledger, err := client.GetOrderLedger(ctx, "test-order")
	require.Error(t, err)
	assert.Nil(t, ledger)
	// Should contain context deadline or cancellation error
	assert.True(t, err != nil)
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		wantErr  bool
	}{
		{
			name:     "positive amount with dollar sign",
			input:    "$178.96",
			expected: 178.96,
			wantErr:  false,
		},
		{
			name:     "negative amount with dollar sign",
			input:    "-$15.00",
			expected: -15.00,
			wantErr:  false,
		},
		{
			name:     "amount with comma separator",
			input:    "$1,234.56",
			expected: 1234.56,
			wantErr:  false,
		},
		{
			name:     "negative amount with comma",
			input:    "-$1,000.00",
			expected: -1000.00,
			wantErr:  false,
		},
		{
			name:     "zero amount",
			input:    "$0.00",
			expected: 0.00,
			wantErr:  false,
		},
		{
			name:     "amount without dollar sign",
			input:    "100.50",
			expected: 100.50,
			wantErr:  false,
		},
		{
			name:     "invalid amount string",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAmount(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExtractLastFour(t *testing.T) {
	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "standard ending in format",
			description: "Ending in 0953",
			expected:    "0953",
		},
		{
			name:        "lowercase ending in",
			description: "ending in 1234",
			expected:    "1234",
		},
		{
			name:        "mixed case",
			description: "ENDING IN 5678",
			expected:    "5678",
		},
		{
			name:        "walmart cash description",
			description: "Walmart Cash",
			expected:    "",
		},
		{
			name:        "empty description",
			description: "",
			expected:    "",
		},
		{
			name:        "no ending in pattern",
			description: "Credit Card",
			expected:    "",
		},
		{
			name:        "ending in with extra spaces",
			description: "Ending  in  9999",
			expected:    "9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLastFour(tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrderLedgerResponseUnmarshaling(t *testing.T) {
	jsonData := `{
		"data": {
			"getOrderLedger": {
				"paymentMethodsLedgers": [
					{
						"paymentType": "CREDITCARD",
						"cardType": "VISA",
						"description": "Ending in 0953",
						"transactions": [
							{
								"chargeType": "FINAL_CHARGES",
								"transactionLines": [
									{
										"date": "December 16, 2024",
										"rowLines": [
											{
												"lineType": "ORDER_CHARGE",
												"displayValues": ["$178.96"],
												"time": "4:31 AM"
											}
										]
									}
								]
							}
						]
					}
				]
			}
		}
	}`

	var response OrderLedgerResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	require.NoError(t, err)

	ledgers := response.Data.GetOrderLedger.PaymentMethodsLedgers
	require.Len(t, ledgers, 1)
	assert.Equal(t, "CREDITCARD", ledgers[0].PaymentType)
	assert.Equal(t, "VISA", ledgers[0].CardType)
	assert.Equal(t, "Ending in 0953", ledgers[0].Description)

	require.Len(t, ledgers[0].Transactions, 1)
	assert.Equal(t, "FINAL_CHARGES", ledgers[0].Transactions[0].ChargeType)

	require.Len(t, ledgers[0].Transactions[0].TransactionLines, 1)
	assert.Equal(t, "December 16, 2024", ledgers[0].Transactions[0].TransactionLines[0].Date)

	require.Len(t, ledgers[0].Transactions[0].TransactionLines[0].RowLines, 1)
	rowLine := ledgers[0].Transactions[0].TransactionLines[0].RowLines[0]
	assert.Equal(t, "ORDER_CHARGE", rowLine.LineType)
	assert.Equal(t, []string{"$178.96"}, rowLine.DisplayValues)
	assert.Equal(t, "4:31 AM", rowLine.Time)
}
