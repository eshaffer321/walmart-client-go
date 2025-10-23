# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/eshaffer321/walmart-client-go/compare/v1.0.2...HEAD
