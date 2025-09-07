# Contributing to Walmart Client

Thank you for your interest in contributing to the Walmart Client project!

## Development Setup

1. **Clone the repository**
```bash
git clone https://github.com/eshaffer321/walmart-client-go
cd walmart-client-go
```

2. **Install development tools**
```bash
make install-tools
```

## Development Workflow

### Running Tests
```bash
make test
```

### Running Linter
```bash
make lint
```

### Format Code
```bash
make fmt
```

### Run All Checks (before committing)
```bash
make pre-commit
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linting (`make pre-commit`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to your fork (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Code Style

- Follow Go best practices and idioms
- Use `gofmt` and `goimports` for formatting
- Add tests for new functionality
- Keep functions small and focused
- Document exported functions and types

## Testing

- Write unit tests for new features
- Ensure all tests pass before submitting PR
- Aim for >80% code coverage
- Use table-driven tests where appropriate

## Commit Messages

- Use clear, descriptive commit messages
- Start with a verb in present tense
- Keep the first line under 72 characters
- Add detailed description if needed

Example:
```
Add support for filtering orders by date range

- Add MinTimestamp and MaxTimestamp to PurchaseHistoryRequest
- Update buildPurchaseHistoryEndpoint to include date filters
- Add tests for date filtering functionality
```