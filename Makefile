.PHONY: all build test clean lint fmt vet security install-tools coverage

# Variables
BINARY_NAME=walmart-cli
CMD_DIR=./cmd/walmart
COVERAGE_FILE=coverage.out

all: clean lint test build

build:
	@echo "Building..."
	go build -v -o $(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BINARY_NAME)"

test:
	@echo "Running tests..."
	go test -v -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "Cleaning..."
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE) coverage.html
	rm -rf dist/

lint:
	@echo "Running linter..."
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		if [ -f ~/go/bin/golangci-lint ]; then \
			~/go/bin/golangci-lint run ./...; \
		else \
			echo "golangci-lint not installed. Run: make install-tools" && exit 1; \
		fi \
	else \
		golangci-lint run ./...; \
	fi

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet:
	@echo "Running go vet..."
	go vet ./...

security:
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "gosec not installed. Run: make install-tools" && exit 1)
	gosec -fmt json -out gosec-report.json ./...
	@echo "Security report generated: gosec-report.json"

install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installed successfully"

# Run before committing
pre-commit: fmt lint test
	@echo "Pre-commit checks passed!"

# Watch for changes and run tests
watch:
	@which entr > /dev/null || (echo "entr not installed. Install it with: brew install entr (macOS) or apt-get install entr (Linux)" && exit 1)
	find . -name "*.go" | entr -c make test

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Check for outdated dependencies
deps-check:
	@echo "Checking dependencies..."
	go list -u -m all

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Generate documentation
docs:
	@echo "Generating documentation..."
	@which godoc > /dev/null || go install golang.org/x/tools/cmd/godoc@latest
	@echo "Starting godoc server on http://localhost:6060"
	godoc -http=:6060

help:
	@echo "Available targets:"
	@echo "  make build         - Build the CLI binary"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make security      - Run security scan"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install-tools - Install development tools"
	@echo "  make pre-commit    - Run pre-commit checks"
	@echo "  make watch         - Watch for changes and run tests"
	@echo "  make bench         - Run benchmarks"
	@echo "  make deps-check    - Check for outdated dependencies"
	@echo "  make deps-update   - Update dependencies"
	@echo "  make docs          - Generate and serve documentation"
	@echo "  make help          - Show this help message"