# Walmart API Client

[![CI](https://github.com/eshaffer321/walmart-client-go/actions/workflows/ci.yml/badge.svg)](https://github.com/eshaffer321/walmart-client-go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/eshaffer321/walmart-client-go/branch/main/graph/badge.svg)](https://codecov.io/gh/eshaffer321/walmart-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/eshaffer321/walmart-client)](https://goreportcard.com/report/github.com/eshaffer321/walmart-client)
[![Go Reference](https://pkg.go.dev/badge/github.com/eshaffer321/walmart-client.svg)](https://pkg.go.dev/github.com/eshaffer321/walmart-client)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A robust Go library and CLI for accessing Walmart order history and purchase data through their GraphQL API.

Available as both a **Go library** for programmatic access and a **CLI tool** for command-line usage.

## Features

- 🛒 Fetch complete order history (both in-store and delivery orders)
- 📦 Get detailed order information including items, prices, tax, and totals
- 💰 **NEW:** Driver tip tracking for delivery orders - see actual amount charged
- 🔍 Search orders for specific items
- 📄 Pagination support for large order histories
- 🍪 Automatic cookie management with rotation to prevent staleness
- 💾 Persistent cookie storage in `~/.walmart-api/cookies.json`

## Installation

### As a Go Library
```bash
go get github.com/eshaffer321/walmart-client
```

### As a CLI Tool
```bash
# Clone and build
git clone https://github.com/eshaffer321/walmart-client-go
cd walmart-client-go
go build -o walmart-cli ./cmd/walmart

# Or install directly
go install github.com/eshaffer321/walmart-client/cmd/walmart@latest
```

## Library Usage (Go SDK)

### Quick Start
```go
package main

import (
    "encoding/json"
    "fmt"
    "time"
    
    walmart "github.com/eshaffer321/walmart-client"
)

func main() {
    // Initialize client
    config := walmart.ClientConfig{
        RateLimit: 2 * time.Second,
        AutoSave:  true,
    }
    
    client, err := walmart.NewWalmartClient(config)
    if err != nil {
        panic(err)
    }
    
    // Initialize from curl file (one-time setup)
    err = client.InitializeFromCurl("curl.txt")
    if err != nil {
        panic(err)
    }
    
    // Get recent orders as Go structs
    orders, err := client.GetRecentOrders(10)
    if err != nil {
        panic(err)
    }
    
    // Access data programmatically
    for _, order := range orders {
        fmt.Printf("Order %s: %d items\n", order.OrderID, order.ItemCount)
    }
    
    // Get full order details
    if len(orders) > 0 {
        order, err := client.GetOrder(orders[0].OrderID, true)
        if err != nil {
            panic(err)
        }
        
        // Access as structured data
        fmt.Printf("Total: %.2f\n", order.PriceDetails.GrandTotal.Value)
        fmt.Printf("Tax: %.2f\n", order.PriceDetails.TaxTotal.Value)
        
        // Or convert to JSON
        jsonData, _ := json.MarshalIndent(order, "", "  ")
        fmt.Println(string(jsonData))
    }
}
```

### Available Methods
```go
// Order operations
client.GetOrder(orderID string, isInStore bool) (*Order, error)
client.GetOrderAutoDetect(orderID string) (*Order, error)
client.GetDeliveryOrderWithTip(orderID string) (*Order, error) // NEW: Ensures tip info is included

// Purchase history
client.GetRecentOrders(limit int) ([]OrderSummary, error)
client.GetAllOrders(maxPages int) ([]OrderSummary, error)
client.SearchOrders(searchTerm string, limit int) ([]OrderSummary, error)
client.GetOrdersByType(orderType string, limit int) ([]OrderSummary, error)

// Cookie management
client.InitializeFromCurl(curlFile string) error
client.Status() // Print status
client.RefreshFromBrowser() error

// Helper methods for JSON output
client.GetOrdersAsJSON(limit int) (string, error)
client.GetOrderAsJSON(orderID string, isInStore bool) (string, error)
```

### Data Structures
All responses return strongly-typed Go structs with JSON tags:

```go
type Order struct {
    ID             string               `json:"id"`
    OrderDate      string               `json:"orderDate"`
    DisplayID      string               `json:"displayId"`
    Groups         []OrderGroup         `json:"groups_2101"`
    PriceDetails   *OrderPriceDetails   `json:"priceDetails"`
    PaymentMethods []OrderPaymentMethod `json:"paymentMethods"`
    // ... more fields
}

type OrderPriceDetails struct {
    SubTotal     *PriceLineItem  `json:"subTotal"`
    TaxTotal     *PriceLineItem  `json:"taxTotal"`
    GrandTotal   *PriceLineItem  `json:"grandTotal"`
    DriverTip    *PriceLineItem  `json:"driverTip"`    // NEW: Driver tip for delivery
    TotalWithTip *PriceLineItem  `json:"totalWithTip"` // NEW: Total including tip
    Savings      *PriceLineItem  `json:"savings"`
    Fees         []PriceLineItem `json:"fees"`         // NEW: Additional fees
}
```

### Working with Delivery Orders and Tips

For delivery orders, the client now tracks driver tips to match the actual card charge:

```go
// Fetch a delivery order with tip information
order, err := client.GetDeliveryOrderWithTip("200013441152420")
if err != nil {
    log.Fatal(err)
}

// Access pricing with tip
if order.PriceDetails != nil {
    fmt.Printf("Subtotal: $%.2f\n", order.PriceDetails.SubTotal.Value)
    fmt.Printf("Tax: $%.2f\n", order.PriceDetails.TaxTotal.Value)
    fmt.Printf("Grand Total: $%.2f\n", order.PriceDetails.GrandTotal.Value)
    
    // Driver tip (if available in API response)
    if order.PriceDetails.DriverTip != nil {
        fmt.Printf("Driver Tip: $%.2f\n", order.PriceDetails.DriverTip.Value)
    }
    
    // Total including tip (calculated automatically)
    if order.PriceDetails.TotalWithTip != nil {
        fmt.Printf("Total with Tip: $%.2f\n", order.PriceDetails.TotalWithTip.Value)
        fmt.Println("This should match your credit card charge")
    }
}

// Check if an order is a delivery order
if order.IsDeliveryOrder() {
    fmt.Println("This is a delivery order")
}
```

## CLI Usage

### Setup

1. **Get your cookies from Walmart.com:**
   - Log into walmart.com in Chrome/Firefox
   - Go to your orders page
   - Open DevTools (F12) → Network tab
   - Refresh the page
   - Find any 'getOrder' request
   - Right-click → Copy → Copy as cURL
   - Save to a file (e.g., `curl.txt`)

2. **Initialize the CLI:**
```bash
./walmart-cli -init curl.txt
```

This saves your cookies to `~/.walmart-api/cookies.json` for future use.

### CLI Commands

#### View Recent Orders
```bash
./walmart-cli -history

# Output:
# === Order History (10 orders) ===
# 
# 1. Order #18420337004257359578
#    Type: IN_STORE | Status: IN_STORE
#    Date: Sep 05, 2025 purchase
#    Store: MERIDIAN Supercenter
#    Items (3):
#      - Great Value Cracker Cut Sliced 4 Cheese Tray, 16 oz (qty: 1)
#      ...
```

#### Search Orders
```bash
./walmart-cli -search "cheese"
```

#### Get Order Details
```bash
./walmart-cli -order 18420337004257359578

# Output:
# === Order Details ===
# Order ID:     18420337004257359578
# Display ID:   1842-0337-0042-5735-9578
# Date:         Sep 5, 2025 at 4:16 PM
# 
# Items (3):
#   1. Great Value Cracker Cut Sliced 4 Cheese Tray, 16 oz
#      Item #814783251
#      Qty: 1 = $4.98
#   ...
# 
# === Price Summary ===
# Subtotal:     $7.14
# Tax:          $0.43
# Total:        $7.57
# 
# === Payment ===
# Visa ending in 0953
```

#### List All Orders (with pagination)
```bash
./walmart-cli -list-all
```

#### Check Cookie Status
```bash
./walmart-cli -status

# Output:
# === Cookie Store Status ===
# Total cookies: 61
# Cookie file: /Users/you/.walmart-api/cookies.json
# Essential cookies: 6
# 
# Essential cookies:
#   ✅ CID: 2m30s ago
#   ✅ SPID: 2m30s ago
#   ✅ auth: 2m30s ago
#   ✅ customer: 2m30s ago
```

#### Refresh Cookies
```bash
./walmart-cli -refresh
# Follow prompts to update cookies from browser
```

## How It Works

### Authentication
- Uses 61 cookies from your browser session
- **CID** and **SPID** are the essential auth cookies
- However, ALL 61 cookies are required to avoid bot detection (429/418 errors)
- 19 cookies automatically update with each request to prevent staleness

### API Endpoints

1. **Purchase History** (`PurchaseHistoryV2`)
   - Hash: `2c3d5a832b56671dca1ed0ec84940f274d0bc80821db4ad7481e496c0ad5847e`
   - Returns all order types (IN_STORE, DELIVERY, PICKUP)
   - Supports pagination with cursor
   - Filtering by date range, order type, search terms

2. **Order Details** (`getOrder`)
   - Hash: `d0622497daef19150438d07c506739d451cad6749cf45c3b4db95f2f5a0a65c4`
   - Returns complete order information
   - Automatically detects if order is IN_STORE or DELIVERY

### Data Available

Each order includes:
- ✅ Order ID, date, and display ID
- ✅ Complete item list with names, quantities, and item IDs
- ✅ Individual item prices
- ✅ Subtotal, tax, and total amounts
- ✅ **Driver tips for delivery orders (when available)**
- ✅ **Total with tip - matches actual card charge**
- ✅ Payment method information
- ✅ Store information (for in-store purchases)
- ✅ Delivery details (for online orders)

## File Structure

```
walmart-client/
├── client.go            # Main client with cookie management
├── models.go            # Data structures for orders
├── purchase_history.go  # Purchase history API methods
├── example_usage.go     # Library usage examples
├── example_json.go      # JSON conversion helpers
├── cmd/
│   └── walmart/
│       └── main.go      # CLI interface
└── example/
    └── main.go          # Example usage
```

## Cookie Storage

Cookies are stored in `~/.walmart-api/cookies.json` with metadata:
```json
{
  "cookies": {
    "CID": {
      "value": "...",
      "last_update": "2025-09-07T08:40:57Z",
      "source": "curl",
      "essential": true
    },
    ...
  },
  "last_update": "2025-09-07T08:40:57Z"
}
```

## Technical Details

### Rate Limiting
- Built-in 2-second delay between requests
- Automatic cookie updates to prevent staleness
- Proper error handling for rate limits (429) and bot detection (418)

### GraphQL Persisted Queries
Walmart uses persisted queries where the query is stored server-side and referenced by hash:
- Each operation type has a unique hash
- Client sends hash + variables instead of full query
- Reduces bandwidth and hides query complexity

### Order Types
- **IN_STORE**: Physical store purchases (`orderIsInStore: true`)
- **DELIVERY**: Online orders delivered to home (`orderIsInStore: false`)
- **PICKUP**: Online orders picked up at store

## Notes

- This is for personal use only - be respectful of Walmart's servers
- Cookies expire after some time - refresh from browser when needed
- Rate limiting is enforced to avoid detection
- All 61 cookies are required despite only 2 containing auth data

## License

For personal use only. This tool is designed for accessing your own order history.