# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.0] - 2025-12-24

### Breaking Changes
- **All public API methods now require `context.Context` as the first parameter**
  - This enables proper cancellation, timeouts, and graceful shutdown
  - Affected methods:
    - `GetPurchaseHistory(ctx context.Context, req PurchaseHistoryRequest)`
    - `GetRecentOrders(ctx context.Context, limit int)`
    - `GetAllOrders(ctx context.Context, maxPages int)`
    - `SearchOrders(ctx context.Context, searchTerm string, limit int)`
    - `GetOrdersByType(ctx context.Context, orderType string, limit int)`
    - `GetOrder(ctx context.Context, orderID string, isInStore bool)`
    - `GetOrderAutoDetect(ctx context.Context, orderID string)`
    - `GetDeliveryOrderWithTip(ctx context.Context, orderID string)`
    - `GetOrdersAsJSON(ctx context.Context, limit int)`
    - `GetOrderAsJSON(ctx context.Context, orderID string, isInStore bool)`
    - `GetOrderLedger(ctx context.Context, orderID string)`

### Added
- **Context support for cancellation and timeouts** across all API methods
  - Rate limiter waits are now cancellable via context
  - HTTP requests use `http.NewRequestWithContext()` for proper cancellation
  - Exponential backoff in `GetOrderLedger` retries is now cancellable
- **Context cancellation test** (`TestGetOrderLedgerContextCancellation`)
- Improved daemon mode with graceful shutdown via SIGINT/SIGTERM

### Changed
- Rate limiting now uses `select` with context for interruptible waits
- `GetAllOrders` checks for context cancellation between page fetches
- `GetOrderAutoDetect` checks for cancellation before retry attempt
- Replaced `fmt.Printf` pagination logging in `GetAllOrders` with structured `logger.Debug`
- CLI commands now use timeouts (2-10 minutes depending on operation)

### Migration Guide
To migrate from v1.x to v2.0.0, add `context.Context` as the first parameter to all API calls:

```go
// Before (v1.x)
orders, err := client.GetRecentOrders(10)
order, err := client.GetOrder(orderID, isInStore)
ledger, err := client.GetOrderLedger(orderID)

// After (v2.0.0)
ctx := context.Background()
orders, err := client.GetRecentOrders(ctx, 10)
order, err := client.GetOrder(ctx, orderID, isInStore)
ledger, err := client.GetOrderLedger(ctx, orderID)

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
orders, err := client.GetRecentOrders(ctx, 10)
```

## [1.0.9] - 2025-12-24

### Added
- **ChargedDates field** in `PaymentMethodCharges` struct to track when each charge occurred
- Parallel to `FinalCharges` slice - each date corresponds to a charge amount
- New `parseChargeDateTime()` function to parse Walmart's date formats ("Dec 23, 2025 4:31 AM")
- Supports both abbreviated and full month formats with graceful fallback

## [1.0.8] - 2025-10-27

### Changed
- Refactored logger to remove redundant `client` attribute from logger (#12)
- Simplified logging setup by removing per-call attribute injection

## [1.0.7] - 2025-10-26

### Fixed
- **CRITICAL:** Added missing `setCookies()` call in `GetOrderLedger()`
- Ledger requests were being sent without authentication cookies
- This caused Walmart's bot detection to return empty responses
- Multi-delivery order tracking now works correctly with populated payment methods
- Matches the pattern used in `GetOrder()` and other working endpoints

## [1.0.6] - 2025-10-25

### Added
- **New Config Options** for better ledger API handling:
  - `LedgerRateLimit`: Separate rate limit for ledger API (stricter than regular rate limit)
  - `MaxRetries`: Configurable retry count with exponential backoff (default: 3)
- **Automatic Retry Logic** with exponential backoff for 429 errors in `GetOrderLedger()`
  - Retries with delays: 5s, 10s, 20s, 40s, etc.
  - Helps handle Walmart's stricter rate limits on the ledger endpoint

### Changed
- `GetOrderLedger()` now uses a separate ledger-specific rate limiter
- Better logging for retry attempts and backoff periods
- Updated tests to handle new retry behavior

### Fixed
- Addresses persistent 429 errors when calling `GetOrderLedger()` multiple times
- Ledger endpoint no longer shares rate limit budget with order fetching
- More resilient to temporary rate limiting

## [1.0.5] - 2025-10-25

### Fixed
- **CRITICAL:** Fixed `GetOrderLedger()` to send correct operation name in headers
- Changed `x-apollo-operation-name` from "getOrder" to "getOrderLedger"
- Changed `x-o-gql-query` from "query getOrder" to "query getOrderLedger"
- Made `setHeaders()` accept operation name as parameter for flexibility
- This was the second part of the fix - v1.0.4 fixed URL encoding, this fixes headers
- **Tested with real API** - confirmed working with 200 OK response

## [1.0.4] - 2025-10-25

### Fixed
- **CRITICAL:** Fixed `GetOrderLedger()` URL construction to properly URL-encode JSON variables
- Variables parameter now uses `url.Values` and `params.Encode()` like other working endpoints
- This was the root cause of 429 errors - malformed URLs were rejected by Walmart's API
- Previous fix (v1.0.3) added rate limiting but didn't address the URL encoding issue

## [1.0.3] - 2025-10-25

### Fixed
- Added rate limiting to `GetOrderLedger()` to prevent 429 (Too Many Requests) errors (#10)
- Added proper error handling for 429 and 403/418 status codes in `GetOrderLedger()`
- Added cookie management to `GetOrderLedger()` to maintain fresh session state
- Fixed issue where ledger requests would fail immediately after successful order fetches

## [1.0.2] - 2025-10-22

### Changed
- Moved documentation files to `docs/` directory for cleaner root
- Moved `CLAUDE.md` to `docs/CLAUDE.md`
- Moved `RELEASING.md` to `docs/RELEASING.md`
- Moved `MIGRATION.md` to `docs/MIGRATION.md`

### Removed
- `walmart-cli` binary (was mistakenly committed to git)
- `RESTRUCTURE_SUMMARY.md` (temporary documentation artifact)
- `CONTRIBUTING.md` (unnecessary for personal project)

## [1.0.1] - 2025-10-20

### Changed
- Reorganized package structure for better maintainability
- Moved `CookieStore` and `Cookie` types to `internal/cookies/` package
- Split `client.go` into logical files (`client.go`, `config.go`, `orders.go`)
- Renamed files for clarity (`purchase_history.go` → `purchases.go`, `orderledger.go` → `ledger.go`)
- Improved file organization with `examples/`, `testdata/`, and `internal/` directories

### Added
- Package-level documentation in `doc.go`
- `CLAUDE.md` - AI/human maintenance guide with release process
- `CHANGELOG.md` - This file
- `RELEASING.md` - Detailed release process guide
- `MIGRATION.md` - Migration guide for restructured code
- `testdata/` directory for test fixtures
- Proper Go Examples that appear in godoc

### Removed
- `example_usage.go` - Moved to `examples/basic/`
- `example_json.go` - Converted to proper Go examples
- `test_tip.go` - Converted to standard test helpers

### Fixed
- Module path corrected to `github.com/eshaffer321/walmart-client-go`
- All internal imports updated to use correct module path

## [Prehistory] - Pre-v1.0.0

The following changes were made during initial development before formal versioning:

### 2025-10-19 - Logging Support
- Added optional logger injection with structured logging (#6)
- Support for `log/slog` with JSON and text formats
- All logs include `client=walmart` attribute for filtering

### 2025-10-17 - Order Ledger API
- Added `GetOrderLedger()` method for payment tracking (#4)
- Support for reconciling orders with bank transactions
- Track split charges and multiple payment methods

### 2025-09-21 - Client Enhancements
- Enhanced client functionality and improved documentation (#3)
- Fixed decimal quantity parsing for weighted items (#2)
- Better handling of fractional quantities

### 2025-09-07 - Initial Release
- Core Walmart API client with cookie management
- `GetOrder()`, `GetRecentOrders()`, `GetAllOrders()` methods
- Purchase history pagination support
- Automatic cookie rotation to prevent staleness
- CLI tool for command-line usage
- Comprehensive test suite
- CI/CD pipeline with Go 1.21, 1.22, 1.23 testing
- Code coverage reporting via Codecov

## Release Naming Convention

- **v1.x.x** - Current stable release series
- **v1.0.2** - Latest (documentation cleanup)
- **v1.0.1** - Clean architecture with internal packages
- **Pre-v1.0.1** - Untagged development commits

## How to Release

See [docs/RELEASING.md](docs/RELEASING.md) and [docs/CLAUDE.md](docs/CLAUDE.md) for detailed release instructions.

Quick reference:
```bash
# 1. Update this file (move Unreleased to vX.Y.Z)
# 2. Run checks
make pre-commit

# 3. Tag and push
git tag -a v1.0.3 -m "Release v1.0.3: Description"
git push origin v1.0.3

# 4. Create GitHub release
gh release create v1.0.3 --generate-notes
```

[Unreleased]: https://github.com/eshaffer321/walmart-client-go/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.9...v2.0.0
[1.0.9]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.8...v1.0.9
[1.0.8]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.7...v1.0.8
[1.0.7]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.5...v1.0.6
[1.0.5]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.2...v1.0.3
