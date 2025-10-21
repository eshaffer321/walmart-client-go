# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **BREAKING:** Reorganized package structure for better maintainability
- **BREAKING:** Moved `CookieStore` and `Cookie` types to `internal/cookies/` package
- **BREAKING:** Removed example functions (`ExampleUsage`, `ExampleJSON`) from public API
- Split `client.go` into logical files (`client.go`, `config.go`, `orders.go`)
- Renamed files for clarity (`purchase_history.go` → `purchases.go`, `orderledger.go` → `ledger.go`)
- Improved file organization with `examples/`, `testdata/`, and `internal/` directories

### Added
- Package-level documentation in `doc.go`
- `CLAUDE.md` - AI/human maintenance guide with release process
- `CHANGELOG.md` - This file
- `RELEASING.md` - Detailed release process guide
- `MIGRATION.md` - Migration guide from v1 (pre-restructure) to v2
- `testdata/` directory for test fixtures
- Proper Go Examples that appear in godoc

### Removed
- `example_usage.go` - Moved to `examples/basic/`
- `example_json.go` - Converted to proper Go examples
- `test_tip.go` - Converted to standard test helpers

## [Prehistory] - Pre-v2.0.0

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

- **v2.x.x** - Current clean architecture
- **v1.x.x** - Never officially tagged (pre-restructure commits)

## How to Release

See [RELEASING.md](RELEASING.md) and [CLAUDE.md](CLAUDE.md) for detailed release instructions.

Quick reference:
```bash
# 1. Update this file (move Unreleased to vX.Y.Z)
# 2. Run checks
make pre-commit

# 3. Tag and push
git tag -a v2.0.0 -m "Release v2.0.0: Clean architecture"
git push origin v2.0.0

# 4. Create GitHub release
gh release create v2.0.0 --generate-notes
```

[Unreleased]: https://github.com/eshaffer321/walmart-client-go/compare/v2.0.0...HEAD
