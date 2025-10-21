# Release Process

This guide provides step-by-step instructions for releasing new versions of walmart-client.

## Semantic Versioning

We follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** version (v3.0.0): Incompatible API changes
- **MINOR** version (v2.1.0): New functionality, backward compatible
- **PATCH** version (v2.0.1): Bug fixes, backward compatible

### When to Bump Which Version

**MAJOR (Breaking Changes):**
- Removing or renaming exported functions/types
- Changing function signatures
- Changing behavior in incompatible ways
- Removing support for Go versions

**MINOR (New Features):**
- Adding new exported functions/types
- Adding new fields to structs (if doesn't break existing usage)
- New functionality that's backward compatible

**PATCH (Bug Fixes):**
- Fixing bugs
- Performance improvements
- Documentation fixes
- Internal refactoring (no API changes)

## Release Checklist

### 1. Prepare the Release

#### A. Ensure main branch is ready
```bash
git checkout main
git pull origin main
git status  # Should be clean
```

#### B. Run full test suite
```bash
# Run all tests with race detection
make test

# Run linting
make lint

# Format code
make fmt

# Run full pre-commit checks
make pre-commit
```

All checks must pass before proceeding.

#### C. Update CHANGELOG.md

Open `CHANGELOG.md` and move items from `[Unreleased]` to a new version section:

```markdown
## [Unreleased]

<!-- Empty for now -->

## [2.1.0] - 2025-10-20

### Added
- GetOrderLedger method for payment tracking (#15)
- Support for split payment reconciliation

### Fixed
- Cookie rotation bug causing intermittent auth failures (#14)

### Changed
- Improved error messages for API failures
```

**Format:** `## [X.Y.Z] - YYYY-MM-DD`

#### D. Update version references

Check if version is referenced anywhere else:
```bash
# Search for old version references
rg "v2\.0\.0" --type md
```

Common places:
- README.md (installation examples)
- Documentation files

### 2. Commit the Changes

```bash
git add CHANGELOG.md
git commit -m "Prepare release v2.1.0"
git push origin main
```

### 3. Create the Git Tag

```bash
# Create annotated tag (required for Go modules)
git tag -a v2.1.0 -m "Release v2.1.0: Payment ledger support"

# Verify tag was created
git tag -l | grep v2.1.0

# Push tag to GitHub
git push origin v2.1.0
```

**Important:** Always use annotated tags (`-a` flag), not lightweight tags.

### 4. Create GitHub Release

#### Option A: Using GitHub CLI (Recommended)

```bash
# Auto-generate release notes from commits
gh release create v2.1.0 --generate-notes

# Or provide custom notes
gh release create v2.1.0 \
  --title "v2.1.0 - Payment Ledger Support" \
  --notes "See [CHANGELOG.md](CHANGELOG.md) for details."
```

#### Option B: Using GitHub Web UI

1. Go to https://github.com/eshaffer321/walmart-client-go/releases/new
2. Select the tag: `v2.1.0`
3. Set title: `v2.1.0 - Payment Ledger Support`
4. Copy relevant section from CHANGELOG.md into description
5. Click "Publish release"

### 5. Verify the Release

```bash
# Check tag exists on GitHub
gh release list

# Verify Go can fetch the new version
go list -m github.com/eshaffer321/walmart-client@v2.1.0

# Test installation in temp directory
cd /tmp
mkdir test-release && cd test-release
go mod init test
go get github.com/eshaffer321/walmart-client@v2.1.0
```

Should output:
```
go: downloading github.com/eshaffer321/walmart-client v2.1.0
```

### 6. Update CHANGELOG.md Comparison Links

At the bottom of CHANGELOG.md, add comparison link:

```markdown
[Unreleased]: https://github.com/eshaffer321/walmart-client-go/compare/v2.1.0...HEAD
[2.1.0]: https://github.com/eshaffer321/walmart-client-go/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/eshaffer321/walmart-client-go/releases/tag/v2.0.0
```

Commit and push:
```bash
git add CHANGELOG.md
git commit -m "Update CHANGELOG comparison links"
git push origin main
```

## Hotfix Releases

For urgent bug fixes that need immediate release:

### 1. Create hotfix branch
```bash
git checkout -b hotfix/v2.0.1
```

### 2. Fix the bug
```bash
# Make fix, add tests
git add .
git commit -m "Fix critical cookie expiration bug"
```

### 3. Update CHANGELOG.md
```markdown
## [2.0.1] - 2025-10-21

### Fixed
- Critical bug causing premature cookie expiration (#20)
```

### 4. Merge to main
```bash
git checkout main
git merge hotfix/v2.0.1
git push origin main
```

### 5. Follow normal release process
```bash
git tag -a v2.0.1 -m "Release v2.0.1: Fix cookie expiration bug"
git push origin v2.0.1
gh release create v2.0.1 --generate-notes
```

## Release Automation (Future)

Currently releases are manual. Future improvements:

### Option 1: GitHub Actions Release Workflow

Create `.github/workflows/release.yml`:
- Triggered when tag is pushed
- Runs tests automatically
- Creates GitHub release
- Builds CLI binaries for multiple platforms

### Option 2: GoReleaser

Use [GoReleaser](https://goreleaser.com/) for:
- Cross-platform CLI builds
- Homebrew tap updates
- Docker images
- Automated changelog generation

## Troubleshooting

### "Tag already exists"
```bash
# Delete local tag
git tag -d v2.1.0

# Delete remote tag (careful!)
git push origin :refs/tags/v2.1.0

# Recreate tag
git tag -a v2.1.0 -m "Release v2.1.0"
git push origin v2.1.0
```

### "Go can't find the version"
- Wait a few minutes for pkg.go.dev to index
- Force update: visit https://pkg.go.dev/github.com/eshaffer321/walmart-client@v2.1.0
- Check tag is pushed: `git ls-remote --tags origin`

### "Tests failing in CI"
- Never release if CI is red
- Fix tests first, then retry release
- Can delete tag and recreate after fix

## Post-Release Tasks

- [ ] Verify release appears at https://github.com/eshaffer321/walmart-client-go/releases
- [ ] Check pkg.go.dev updates to new version
- [ ] Test installation: `go get github.com/eshaffer321/walmart-client@latest`
- [ ] Update any dependent projects
- [ ] Announce on social media (optional)
- [ ] Close any GitHub issues fixed in this release

## Quick Reference Commands

```bash
# Full release flow
make pre-commit                                           # Run tests
git add CHANGELOG.md && git commit -m "Prepare vX.Y.Z"   # Update changelog
git push origin main                                      # Push changes
git tag -a vX.Y.Z -m "Release vX.Y.Z"                    # Create tag
git push origin vX.Y.Z                                    # Push tag
gh release create vX.Y.Z --generate-notes                 # Create release

# Verify
gh release list                                           # Check release
go list -m github.com/eshaffer321/walmart-client@vX.Y.Z  # Test version
```

## Help

- For questions about versioning: See [Semantic Versioning](https://semver.org/)
- For changelog format: See [Keep a Changelog](https://keepachangelog.com/)
- For Go module versioning: See [Go Modules Reference](https://go.dev/ref/mod#versions)
- For AI assistance: See [CLAUDE.md](CLAUDE.md)
