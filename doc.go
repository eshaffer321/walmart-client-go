// Package walmart provides a robust Go client for accessing Walmart's order
// history and purchase data through their GraphQL API.
//
// This library enables programmatic access to order information, purchase history,
// payment details, and more through Walmart's internal API endpoints.
//
// # Features
//
//   - Complete order history access (in-store, delivery, and pickup orders)
//   - Detailed order information with items, pricing, and payment details
//   - Driver tip tracking for delivery orders
//   - Order ledger API for payment reconciliation with bank transactions
//   - Search orders by item name or other criteria
//   - Automatic cookie management with rotation to prevent staleness
//   - Persistent cookie storage in ~/.walmart-api/cookies.json
//   - Optional structured logging support with log/slog
//   - Built-in rate limiting to respect API throttling
//
// # Quick Start
//
// Initialize the client with a configuration:
//
//	config := walmart.ClientConfig{
//	    RateLimit: 2 * time.Second,
//	    AutoSave:  true,
//	}
//	client, err := walmart.NewWalmartClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Get your authentication cookies from Walmart.com by copying a request as cURL:
//
//	err = client.InitializeFromCurl("curl.txt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Fetch recent orders with context support for cancellation and timeouts:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//
//	orders, err := client.GetRecentOrders(ctx, 10)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, order := range orders {
//	    fmt.Printf("Order %s: %d items\n", order.OrderID, order.ItemCount)
//	}
//
// Get detailed order information:
//
//	order, err := client.GetOrder(ctx, orderID, true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Total: $%.2f\n", order.PriceDetails.GrandTotal.Value)
//	fmt.Printf("Tax: $%.2f\n", order.PriceDetails.TaxTotal.Value)
//
// # Authentication
//
// The client requires cookies from an authenticated Walmart.com browser session.
// All 61 cookies are required to avoid bot detection (429/418 errors), though
// only CID and SPID contain actual authentication data.
//
// Cookies are automatically updated from API responses and persisted to disk
// for reuse across sessions.
//
// # Logging
//
// The client supports optional structured logging using Go's standard log/slog:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	config := walmart.ClientConfig{
//	    Logger: logger,  // Pass nil to disable logging
//	}
//
// All log messages include a "client=walmart" attribute for filtering in
// multi-service environments.
//
// # Rate Limiting
//
// A default 2-second delay is enforced between requests to prevent rate limiting
// by Walmart's servers. This can be configured via ClientConfig.RateLimit.
//
// # Context Support
//
// All public API methods accept a context.Context as the first parameter,
// enabling proper cancellation, timeouts, and graceful shutdown:
//
//   - Rate limiter waits are cancellable via context
//   - HTTP requests respect context cancellation
//   - Long-running operations like GetAllOrders check cancellation between pages
//
// Recommended timeouts:
//   - Single order/history fetch: 1-2 minutes
//   - Pagination operations (GetAllOrders): 5-10 minutes
//   - Ledger requests: 2-5 minutes (has stricter rate limits)
//
// # Thread Safety
//
// WalmartClient is NOT safe for concurrent use from multiple goroutines.
// The rate limiter state is not protected by a mutex. If you need concurrent
// access, create separate client instances for each goroutine.
//
// # Order Types
//
// The API supports three order types:
//   - IN_STORE: Physical store purchases (requires orderIsInStore: true)
//   - DELIVERY: Online orders delivered to home
//   - PICKUP: Online orders picked up at store
//
// # Examples
//
// For complete examples, see the examples/ directory:
//   - examples/basic/ - Basic usage
//   - examples/ledger/ - Payment ledger reconciliation
//   - examples/logger/ - Structured logging
//
// # Note
//
// This library is for personal use to access your own order history.
// Please be respectful of Walmart's servers and adhere to their terms of service.
package walmart
