# CLAUDE.md - AI/Human Maintenance Guide

This document provides context for AI assistants (like Claude) and human maintainers working on this codebase.

## Project Overview

**walmart-client** is a Go library and CLI for accessing Walmart's order history and purchase data through their GraphQL API. It's a personal project focused on clean code and professional practices.

**Key Facts:**
- No external users (personal project)
- Primary goals: clean code, maintainability, learning
- Dual purpose: Go library + CLI tool
- Uses Walmart's internal GraphQL API (reverse-engineered)

## Architecture & Design Decisions

### Package Structure Philosophy

**Single Package Design:** Everything is in the `walmart` package to keep imports simple:
```go
import "github.com/eshaffer321/walmart-client"

client := walmart.NewWalmartClient(config)
order := walmart.Order{}
```

**Why not subpackages?**
- Library is ~3K LOC - not large enough to justify complex structure
- Single import path is more ergonomic
- Follows Go convention (see `oauth2`, `jwt-go`)

**File Organization:**
- `client.go` - Core `WalmartClient` type and constructor
- `config.go` - Configuration and client options
- `orders.go` - Order-related methods (GetOrder, GetDeliveryOrderWithTip)
- `ledger.go` - Payment ledger API (GetOrderLedger)
- `purchases.go` - Purchase history API (GetRecentOrders, SearchOrders)
- `models.go` - All data structures (Order, OrderLedger, etc.)

**Internal Packages:**
- `internal/cookies/` - Cookie storage and management (hidden from users)
- Implementation details that users should never import directly

### Key Design Patterns

#### 1. Cookie Management
Walmart requires a coherent cookie snapshot and request profile from the same browser session. The cookie store:
- Persists to `~/.walmart-api/cookies.json`
- Replaces the previous snapshot when importing a fresh `getOrder` cURL request
- Persists the active GraphQL hash and allowlisted browser headers
- Keeps cookies returned by responses in a path-aware in-memory jar; they never flatten into or overwrite the persistent browser snapshot
- Tracks metadata (last_update, source, essential flag)

#### 2. Rate Limiting
Built-in 2-second delay prevents 429 errors and respects Walmart's servers.

#### 3. Structured Logging
Uses Go's `log/slog` for optional structured logging:
- All logs include `client=walmart` attribute
- Supports JSON or text format
- Can be disabled by passing `nil` logger

#### 4. Type Safety
All API responses are strongly typed Go structs with JSON tags. No `map[string]interface{}` soup.

## Release Process

### Semantic Versioning

We follow [Semantic Versioning 2.0.0](https://semver.org/):
- **MAJOR** (v2.0.0): Breaking API changes
- **MINOR** (v2.1.0): New features, backward compatible
- **PATCH** (v2.1.1): Bug fixes, backward compatible

### How to Release

#### Step 1: Update CHANGELOG.md

Follow [Keep a Changelog](https://keepachangelog.com/) format:

```markdown
## [Unreleased]

### Added
- New feature description

### Changed
- What changed

### Fixed
- Bug fix description

## [2.1.0] - 2025-10-20

### Added
- GetOrderLedger method for payment tracking
```

Before release, move `[Unreleased]` section to a new version section.

#### Step 2: Run Pre-Release Checks

```bash
# Ensure all tests pass
make test

# Ensure linting passes
make lint

# Ensure code is formatted
make fmt

# Run full pre-commit checks
make pre-commit
```

#### Step 3: Tag the Release

```bash
# Ensure main branch is clean
git checkout main
git pull origin main

# Tag with version (use annotated tag)
git tag -a v2.1.0 -m "Release v2.1.0: Add payment ledger support"

# Push tag to GitHub
git push origin v2.1.0
```

#### Step 4: Create GitHub Release

```bash
# Using GitHub CLI (recommended)
gh release create v2.1.0 \
  --title "v2.1.0 - Payment Ledger Support" \
  --notes-file RELEASE_NOTES.md

# Or manually via GitHub web UI:
# https://github.com/eshaffer321/walmart-client-go/releases/new
```

#### Step 5: Verify Release

Check that:
- Tag appears at https://github.com/eshaffer321/walmart-client-go/tags
- Release shows at https://github.com/eshaffer321/walmart-client-go/releases
- `go get github.com/eshaffer321/walmart-client@v2.1.0` works

### Release Checklist Template

Copy this for each release:

```markdown
## Release vX.Y.Z Checklist

- [ ] All features/fixes merged to main
- [ ] CHANGELOG.md updated (move Unreleased → vX.Y.Z)
- [ ] Tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Code formatted (`make fmt`)
- [ ] Version tag created (`git tag -a vX.Y.Z`)
- [ ] Tag pushed to GitHub
- [ ] GitHub release created
- [ ] Release verified (can `go get` the version)
- [ ] Tweet/announce if significant (optional)
```

## Development Workflow

### Adding a New Feature

1. **Create a branch:**
   ```bash
   git checkout -b feature/order-cancellation
   ```

2. **Implement with tests:**
   - Add method to appropriate file (orders.go, ledger.go, etc.)
   - Add corresponding test in `*_test.go`
   - Update models.go if new types needed

3. **Update documentation:**
   - Add to README.md examples section
   - Update godoc comments
   - Add entry to CHANGELOG.md under `[Unreleased]`

4. **Pre-commit checks:**
   ```bash
   make pre-commit
   ```

5. **Create PR:**
   ```bash
   gh pr create --fill
   ```

### Fixing a Bug

1. **Write a failing test first:**
   ```go
   func TestBugFix(t *testing.T) {
       // Reproduce the bug
   }
   ```

2. **Fix the bug**

3. **Verify test passes:**
   ```bash
   make test
   ```

4. **Update CHANGELOG.md:**
   ```markdown
   ### Fixed
   - Fixed cookie rotation causing auth failures (#123)
   ```

### Making Breaking Changes

Breaking changes require a MAJOR version bump (v2 → v3):

1. **Document the breaking change:**
   - Add to CHANGELOG.md under `### Changed` or `### Removed`
   - Mark with `**BREAKING:**` prefix

2. **Update MIGRATION.md:**
   - Add migration guide from previous version
   - Include code examples showing before/after

3. **Bump major version:**
   ```bash
   git tag -a v3.0.0 -m "Release v3.0.0: Clean API redesign"
   ```

## Testing Strategy

### Test Organization

- `client_test.go` - Core client functionality tests
- `orderledger_test.go` - Order ledger API tests
- `testdata/responses/` - JSON fixtures for mocking responses

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test -v ./...

# Specific test
go test -v -run TestGetOrder
```

### Writing Tests

Use table-driven tests for multiple scenarios:

```go
func TestGetOrder(t *testing.T) {
    tests := []struct{
        name     string
        orderID  string
        inStore  bool
        wantErr  bool
    }{
        {"valid in-store order", "123", true, false},
        {"valid delivery order", "456", false, false},
        {"invalid order", "", true, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Common Maintenance Tasks

### Updating Dependencies

```bash
# Check for outdated dependencies
make deps-check

# Update all dependencies
make deps-update

# Or manually:
go get -u ./...
go mod tidy
```

### Updating Go Version

1. Update `go.mod`:
   ```
   go 1.23
   ```

2. Update `.github/workflows/ci.yml`:
   ```yaml
   go-version: ['1.21', '1.22', '1.23']
   ```

3. Test on all supported versions

### Handling Walmart API Changes

If Walmart changes their API:

1. **Capture new curl request:**
   - Login to walmart.com
   - Open DevTools → Network
   - Copy request as cURL

2. **Refresh the stored request profile:**
   - Run `walmart-cli -init curl.txt` so the current `getOrder` hash and browser headers are captured
   - Static fallback hashes are in `orders.go`, `purchases.go`, and `ledger.go`

3. **Update models if response structure changed:**
   - Add/modify types in `models.go`
   - Update JSON tags to match new response

4. **Add test fixtures:**
   - Save new response JSON to `testdata/responses/`
   - Update tests to handle new structure

## Code Style Guidelines

### Exported vs Unexported

- **Exported** (uppercase): Part of public API
  - `WalmartClient`, `Order`, `GetOrder()`
- **Unexported** (lowercase): Internal implementation
  - `buildOrderEndpoint()`, `setHeaders()`

### Error Handling

Always return descriptive errors:

```go
if resp.StatusCode != 200 {
    return nil, fmt.Errorf("walmart API returned %d: %s", resp.StatusCode, body)
}
```

### Logging

Use structured logging with context:

```go
c.logger.Info("fetching order",
    slog.String("order_id", orderID),
    slog.Bool("in_store", isInStore))
```

### Comments

- Package-level godoc in `doc.go`
- Exported types/functions need godoc comments
- Unexported functions: comment if non-obvious

## Project Health Metrics

### Current State (as of restructure)
- **Lines of Code:** ~3,000
- **Test Coverage:** ~75%
- **Go Version:** 1.21+
- **Dependencies:** Minimal (only testify for tests)
- **CI/CD:** ✅ Multi-version testing, coverage, linting

### Goals
- Maintain >70% test coverage
- Zero security vulnerabilities (gosec)
- All PRs pass CI before merge
- Clean `go vet` and `golangci-lint` output

## Troubleshooting

### "Cookie authentication failed"
- Cookies expire after ~24 hours
- Re-run `walmart-cli -refresh` or extract new curl file
- All 61 cookies required (not just auth cookies)

### "Rate limit exceeded (429)"
- Built-in 2-second delay should prevent this
- If happening, increase `RateLimit` in config
- Walmart may have tightened rate limits

### "Tests failing after Walmart API change"
- Capture new API responses
- Update models.go types
- Update testdata fixtures
- May need to update GraphQL hashes

## Resources

- **Go Docs:** https://pkg.go.dev/github.com/eshaffer321/walmart-client
- **Issues:** https://github.com/eshaffer321/walmart-client-go/issues
- **CI/CD:** https://github.com/eshaffer321/walmart-client-go/actions

## For AI Assistants

When working on this codebase:

1. **Maintain backward compatibility** unless explicitly asked to break it
2. **Update CHANGELOG.md** for any user-facing changes
3. **Write tests** for new features
4. **Keep the flat structure** - resist urge to over-engineer with subpackages
5. **Update this file** if you make architectural changes
6. **Follow existing patterns** - consistency > cleverness

## Questions?

This is a personal project, so there's flexibility. When in doubt:
- Prioritize clean, readable code
- Maintain good test coverage
- Keep dependencies minimal
- Document why, not just what
