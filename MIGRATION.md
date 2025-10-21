# Migration Guide: v1 → v2

This guide helps you migrate from the pre-v2.0.0 codebase (untagged commits) to v2.0.0+ with the clean architecture.

## Overview of Changes

v2.0.0 reorganized the codebase for better maintainability while **keeping the core API mostly the same**. The main changes are:

1. **Internal packages:** `CookieStore` moved to `internal/cookies/`
2. **Removed example functions:** `ExampleUsage()` and `ExampleJSON()` removed from public API
3. **File reorganization:** Better logical grouping of code
4. **Improved documentation:** Package docs, examples directory

## Breaking Changes

### 1. CookieStore is Now Internal

**v1 (pre-restructure):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// Direct access to cookie store
store := &walmart.CookieStore{
    Cookies: make(map[string]*walmart.Cookie),
}
```

**v2 (post-restructure):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// CookieStore is internal - use client methods instead
client, _ := walmart.NewWalmartClient(walmart.ClientConfig{})

// Access cookies through client
client.Status()  // View cookie status
client.RefreshFromBrowser()  // Refresh cookies
```

**Migration:** If you were directly manipulating `CookieStore`, switch to using `WalmartClient` methods. The cookie store is now managed internally.

### 2. Example Functions Removed

**v1 (pre-restructure):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// These were exported functions
walmart.ExampleUsage()
walmart.ExampleJSON()
```

**v2 (post-restructure):**
```go
// These functions are removed from the library
// See examples/ directory instead:
// - examples/basic/main.go
// - examples/ledger/main.go
// - examples/logger/main.go
```

**Migration:** If you were calling these functions, copy the code from `examples/` directory instead.

### 3. Test Helpers Removed from Public API

**v1 (pre-restructure):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// These were accidentally exported
walmart.TestDeliveryOrderWithTip()
walmart.TestOrdersWithTips()
```

**v2 (post-restructure):**
```go
// These are now properly internal test functions
// Not accessible from library imports
```

**Migration:** These were never meant to be public API. If you were using them, they're now internal to the test suite.

## What Stayed the Same (No Migration Needed)

### Core Client API - Unchanged ✅

All main client methods work exactly the same:

```go
import walmart "github.com/eshaffer321/walmart-client"

// All these work identically in v1 and v2
client, err := walmart.NewWalmartClient(config)

orders, err := client.GetRecentOrders(10)
allOrders, err := client.GetAllOrders(5)
order, err := client.GetOrder(orderID, isInStore)
order, err := client.GetOrderAutoDetect(orderID)
order, err := client.GetDeliveryOrderWithTip(orderID)
ledger, err := client.GetOrderLedger(orderID)

results, err := client.SearchOrders("cheese", 10)
orders, err := client.GetOrdersByType("DELIVERY", 20)
```

### Data Models - Unchanged ✅

All types work the same:

```go
import walmart "github.com/eshaffer321/walmart-client"

// All these types are identical in v1 and v2
var order walmart.Order
var ledger walmart.OrderLedger
var summary walmart.OrderSummary
var config walmart.ClientConfig
```

### Configuration - Unchanged ✅

```go
import (
    "time"
    "log/slog"
    walmart "github.com/eshaffer321/walmart-client"
)

// ClientConfig works identically
config := walmart.ClientConfig{
    RateLimit: 2 * time.Second,
    AutoSave:  true,
    Logger:    slog.Default(),
}
```

## Step-by-Step Migration

### For Library Users

#### Step 1: Update import
```bash
# Update your go.mod to v2
go get github.com/eshaffer321/walmart-client@v2.0.0
```

#### Step 2: Review your code

Check if you use any of these patterns:

❌ **Direct CookieStore manipulation:**
```go
// v1 - Don't do this anymore
store := &walmart.CookieStore{}
store.Set("CID", &walmart.Cookie{Value: "..."})
```

✅ **Use client methods instead:**
```go
// v2 - Do this
client, _ := walmart.NewWalmartClient(config)
client.InitializeFromCurl("curl.txt")
```

❌ **Calling example functions:**
```go
// v1 - These are removed
walmart.ExampleUsage()
walmart.ExampleJSON()
```

✅ **Use examples directory:**
```bash
# v2 - Run actual examples
cd examples/basic
go run main.go
```

#### Step 3: Run your tests
```bash
go test ./...
```

If tests pass, migration is complete!

### For CLI Users

**No changes needed!** The CLI (`walmart-cli`) works identically:

```bash
# All commands work the same in v2
walmart-cli -history
walmart-cli -order 123456
walmart-cli -search "cheese"
walmart-cli -status
```

## Detailed API Comparison

### Client Methods (No Changes)

| Method | v1 | v2 | Notes |
|--------|----|----|-------|
| `NewWalmartClient()` | ✅ | ✅ | Identical |
| `GetOrder()` | ✅ | ✅ | Identical |
| `GetOrderAutoDetect()` | ✅ | ✅ | Identical |
| `GetDeliveryOrderWithTip()` | ✅ | ✅ | Identical |
| `GetOrderLedger()` | ✅ | ✅ | Identical |
| `GetRecentOrders()` | ✅ | ✅ | Identical |
| `GetAllOrders()` | ✅ | ✅ | Identical |
| `SearchOrders()` | ✅ | ✅ | Identical |
| `GetOrdersByType()` | ✅ | ✅ | Identical |
| `InitializeFromCurl()` | ✅ | ✅ | Identical |
| `RefreshFromBrowser()` | ✅ | ✅ | Identical |
| `Status()` | ✅ | ✅ | Identical |
| `GetOrdersAsJSON()` | ✅ | ✅ | Identical |
| `GetOrderAsJSON()` | ✅ | ✅ | Identical |

### Types (No Changes)

| Type | v1 | v2 | Notes |
|------|----|----|-------|
| `WalmartClient` | ✅ | ✅ | Identical |
| `ClientConfig` | ✅ | ✅ | Identical |
| `Order` | ✅ | ✅ | Identical |
| `OrderLedger` | ✅ | ✅ | Identical |
| `OrderSummary` | ✅ | ✅ | Identical |
| `OrderPriceDetails` | ✅ | ✅ | Identical |
| `OrderPaymentMethod` | ✅ | ✅ | Identical |
| `CookieStore` | ✅ | ❌ | Moved to `internal/` |
| `Cookie` | ✅ | ❌ | Moved to `internal/` |

### Functions (Removed)

| Function | v1 | v2 | Migration |
|----------|----|----|-----------|
| `ExampleUsage()` | ✅ | ❌ | See `examples/basic/main.go` |
| `ExampleJSON()` | ✅ | ❌ | See `examples/` directory |
| `TestDeliveryOrderWithTip()` | ✅ | ❌ | Was internal test, removed |
| `TestOrdersWithTips()` | ✅ | ❌ | Was internal test, removed |

## Common Migration Scenarios

### Scenario 1: Basic Usage (No Changes Needed)

If your code looks like this, **no migration needed**:

```go
package main

import (
    "fmt"
    "time"
    walmart "github.com/eshaffer321/walmart-client"
)

func main() {
    config := walmart.ClientConfig{
        RateLimit: 2 * time.Second,
        AutoSave:  true,
    }

    client, err := walmart.NewWalmartClient(config)
    if err != nil {
        panic(err)
    }

    orders, err := client.GetRecentOrders(10)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Found %d orders\n", len(orders))
}
```

This works identically in v1 and v2.

### Scenario 2: Using CookieStore Directly (Needs Migration)

**Before (v1):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// Directly creating cookie store
store := &walmart.CookieStore{
    Cookies:  make(map[string]*walmart.Cookie),
    FilePath: "/path/to/cookies.json",
}

cookie := &walmart.Cookie{
    Value: "abc123",
    Essential: true,
}
store.Set("CID", cookie)
```

**After (v2):**
```go
import walmart "github.com/eshaffer321/walmart-client"

// Let the client manage cookies
config := walmart.ClientConfig{
    CookieFile: "/path/to/cookies.json",
}
client, _ := walmart.NewWalmartClient(config)

// Initialize cookies from curl file
client.InitializeFromCurl("curl.txt")

// Or refresh from browser
client.RefreshFromBrowser()
```

### Scenario 3: Using Example Functions (Needs Migration)

**Before (v1):**
```go
import walmart "github.com/eshaffer321/walmart-client"

func main() {
    walmart.ExampleUsage()  // Called exported function
}
```

**After (v2):**
```go
// Copy code from examples/basic/main.go instead
// Or run: cd examples/basic && go run main.go
```

## File Location Changes

| v1 Location | v2 Location | Notes |
|-------------|-------------|-------|
| `client.go` | `client.go`, `config.go`, `orders.go` | Split for clarity |
| `purchase_history.go` | `purchases.go` | Renamed |
| `orderledger.go` | `ledger.go` | Renamed |
| `models.go` | `models.go` | Unchanged |
| `example_usage.go` | `examples/basic/main.go` | Moved |
| `example_json.go` | Removed | See examples |
| `test_tip.go` | `client_test.go` | Merged into tests |
| `CookieStore` type | `internal/cookies/store.go` | Made internal |

## Testing Your Migration

After updating to v2, run this checklist:

```bash
# 1. Update dependency
go get github.com/eshaffer321/walmart-client@v2.0.0
go mod tidy

# 2. Build your project
go build ./...

# 3. Run tests
go test ./...

# 4. Check for breaking changes
# If you see errors about:
#   - "CookieStore undefined" → Migrate to client methods
#   - "ExampleUsage undefined" → Remove call or use examples/
#   - "Cookie undefined" → You were using internal types

# 5. Run your application
go run main.go
```

## Rollback Plan

If you need to rollback to v1 (pre-restructure):

```bash
# Pin to last commit before v2 restructure
go get github.com/eshaffer321/walmart-client@ba6189f

# Or use main branch before the tag
go get github.com/eshaffer321/walmart-client@main
```

**Note:** v1 was never officially tagged, so you'll need to use commit SHA.

## Getting Help

- **Issues with migration?** Open an issue: https://github.com/eshaffer321/walmart-client-go/issues
- **Questions?** Check the examples: `examples/` directory
- **API documentation:** https://pkg.go.dev/github.com/eshaffer321/walmart-client

## Summary

**Most users need no changes!** The core API is unchanged.

Only migrate if you were:
- ❌ Directly using `CookieStore` or `Cookie` types
- ❌ Calling `ExampleUsage()` or `ExampleJSON()` functions
- ❌ Using test helper functions

Otherwise, just update to v2 and you're done:

```bash
go get github.com/eshaffer321/walmart-client@v2.0.0
```
