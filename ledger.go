package walmart

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// OrderLedger represents simplified order payment information
type OrderLedger struct {
	OrderID        string                 `json:"orderId"`
	PaymentMethods []PaymentMethodCharges `json:"paymentMethods"`
}

// PaymentMethodCharges contains the final charges for a payment method
type PaymentMethodCharges struct {
	PaymentType  string    `json:"paymentType"`  // "CREDITCARD", "GIFTCARD"
	CardType     string    `json:"cardType"`     // "VISA", "WMTRC", etc.
	LastFour     string    `json:"lastFour"`     // Last 4 digits of card
	FinalCharges []float64 `json:"finalCharges"` // Individual charge amounts
	TotalCharged float64   `json:"totalCharged"` // Sum of final charges
}

// OrderLedgerResponse represents the raw API response
type OrderLedgerResponse struct {
	Data struct {
		GetOrderLedger struct {
			PaymentMethodsLedgers []PaymentMethodLedger `json:"paymentMethodsLedgers"`
		} `json:"getOrderLedger"`
	} `json:"data"`
}

// PaymentMethodLedger represents a payment method's transaction history
type PaymentMethodLedger struct {
	PaymentType  string        `json:"paymentType"` // "CREDITCARD", "GIFTCARD"
	CardType     string        `json:"cardType"`    // "VISA", "WMTRC"
	Description  string        `json:"description"` // "Ending in 0953"
	Transactions []Transaction `json:"transactions"`
}

// Transaction represents a group of charges by type
type Transaction struct {
	ChargeType       string            `json:"chargeType"` // "FINAL_CHARGES", "TEMPORARY_HOLDS"
	TransactionLines []TransactionLine `json:"transactionLines"`
}

// TransactionLine represents transactions on a specific date
type TransactionLine struct {
	Date     string    `json:"date"`
	RowLines []RowLine `json:"rowLines"`
}

// RowLine represents an individual transaction entry
type RowLine struct {
	LineType      string   `json:"lineType"`      // "ORDER_CHARGE", "ORDER_ADJUSTMENT_REFUND", etc.
	DisplayValues []string `json:"displayValues"` // ["$178.96"], ["$4.12"]
	Time          string   `json:"time"`
}

// GetOrderLedger fetches the payment ledger for an order showing actual charges.
// This endpoint has stricter rate limits than other endpoints, so it uses a separate
// rate limiter and implements retry logic with exponential backoff for 429 errors.
func (c *WalmartClient) GetOrderLedger(orderID string) (*OrderLedger, error) {
	if orderID == "" {
		return nil, fmt.Errorf("order ID is required")
	}

	// Use ledger-specific rate limiter (stricter than regular rate limit)
	if !c.lastLedgerRequest.IsZero() {
		c.logger.Debug("waiting for ledger rate limiter")
		<-c.ledgerRateLimiter.C
	}
	c.lastLedgerRequest = time.Now()

	// Use the hash from the provided URL
	const ledgerHash = "1234d48bfc5e62b608c0dae2c5752f31978870456bcf0023bad3988009e70919"

	// Build variables JSON and properly URL-encode it
	variables := map[string]string{
		"orderId": orderID,
	}
	variablesJSON, _ := json.Marshal(variables)
	params := url.Values{}
	params.Set("variables", string(variablesJSON))

	endpoint := fmt.Sprintf("https://www.walmart.com/orchestra/orders/graphql/getOrderLedger/%s?%s",
		ledgerHash, params.Encode())

	// Retry loop with exponential backoff for 429 errors
	var lastErr error
	maxRetries := c.maxRetries
	if maxRetries < 0 {
		maxRetries = 0 // Disable retries if set to -1
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 5s, 10s, 20s, 40s...
			backoff := time.Duration(5*(1<<uint(attempt-1))) * time.Second
			c.logger.Info("retrying after backoff",
				slog.String("order_id", orderID),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries),
				slog.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		c.logger.Debug("fetching order ledger",
			slog.String("order_id", orderID),
			slog.Int("attempt", attempt+1),
			slog.Int("max_attempts", maxRetries+1))

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set headers
		c.setHeaders(req, "getOrderLedger")

		// Set cookies from store
		c.setCookies(req)

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Error("request failed",
				slog.String("order_id", orderID),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		c.logger.Debug("received response",
			slog.String("order_id", orderID),
			slog.Int("status_code", resp.StatusCode),
			slog.Int("attempt", attempt+1))

		// Update cookies from response
		c.updateCookiesFromResponse(resp)

		// Handle 429 - retry with backoff
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limited (attempt %d/%d)", attempt+1, maxRetries+1)
			c.logger.Warn("rate limited, will retry",
				slog.String("order_id", orderID),
				slog.Int("attempt", attempt+1),
				slog.Int("max_attempts", maxRetries+1))
			continue // Retry
		}

		// Handle other non-OK statuses (don't retry)
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == 403 || resp.StatusCode == 418 {
				c.logger.Warn("access denied",
					slog.String("order_id", orderID),
					slog.Int("status_code", resp.StatusCode))
				return nil, fmt.Errorf("access denied (cookies might be stale) - try refreshing from browser")
			}
			c.logger.Warn("non-200 response",
				slog.String("order_id", orderID),
				slog.Int("status_code", resp.StatusCode))
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		// Success! Parse and return the response
		var ledgerResp OrderLedgerResponse
		if err := json.NewDecoder(resp.Body).Decode(&ledgerResp); err != nil {
			c.logger.Error("failed to parse response",
				slog.String("order_id", orderID),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		// Convert to simplified structure
		ledger := convertToOrderLedger(orderID, ledgerResp)

		c.logger.Info("fetched order ledger",
			slog.String("order_id", orderID),
			slog.Int("payment_method_count", len(ledger.PaymentMethods)),
			slog.Int("attempt", attempt+1))

		return ledger, nil
	}

	// All retries exhausted
	c.logger.Error("all retries exhausted",
		slog.String("order_id", orderID),
		slog.Int("max_retries", maxRetries))
	if lastErr != nil {
		return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
	}
	return nil, fmt.Errorf("failed after %d attempts", maxRetries+1)
}

// convertToOrderLedger converts the raw API response to a simplified structure
func convertToOrderLedger(orderID string, resp OrderLedgerResponse) *OrderLedger {
	ledger := &OrderLedger{
		OrderID:        orderID,
		PaymentMethods: []PaymentMethodCharges{},
	}

	for _, pm := range resp.Data.GetOrderLedger.PaymentMethodsLedgers {
		charges := PaymentMethodCharges{
			PaymentType:  pm.PaymentType,
			CardType:     pm.CardType,
			LastFour:     extractLastFour(pm.Description),
			FinalCharges: []float64{},
		}

		// Extract final charges only
		for _, transaction := range pm.Transactions {
			if transaction.ChargeType == "FINAL_CHARGES" {
				for _, line := range transaction.TransactionLines {
					for _, row := range line.RowLines {
						// Process all display values for this row
						for _, value := range row.DisplayValues {
							if amount, err := parseAmount(value); err == nil {
								charges.FinalCharges = append(charges.FinalCharges, amount)
								charges.TotalCharged += amount
							}
						}
					}
				}
			}
		}

		// Only add payment methods that have charges
		if len(charges.FinalCharges) > 0 {
			ledger.PaymentMethods = append(ledger.PaymentMethods, charges)
		}
	}

	return ledger
}

// parseAmount parses a money string like "$178.96" or "-$15.00" to float64
func parseAmount(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty amount string")
	}

	// Remove dollar sign and commas
	cleaned := strings.ReplaceAll(s, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing amount %q: %w", s, err)
	}

	return amount, nil
}

// extractLastFour extracts the last four digits from descriptions like "Ending in 0953"
func extractLastFour(description string) string {
	// Case-insensitive regex to match "ending in XXXX"
	re := regexp.MustCompile(`(?i)ending\s+in\s+(\d{4})`)
	matches := re.FindStringSubmatch(description)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
