# Walmart Client Examples

This directory contains example programs demonstrating how to use the walmart-client library.

## Running Examples

Each example can be run with:

```bash
cd examples/<example-name>
go run main.go
```

## Available Examples

### basic/
Basic usage example showing:
- Client initialization with structured logging
- Fetching recent orders
- Getting order details
- Searching orders

```bash
cd basic && go run main.go
```

### ledger/
Payment ledger example showing:
- Order ledger API usage
- Reconciling orders with bank transactions
- Tracking split charges
- Payment method breakdown

```bash
cd ledger && go run main.go
```

## Prerequisites

All examples require:
1. Valid Walmart cookies in `~/.walmart-api/cookies.json`
2. Or a `curl.txt` file with a cURL command copied from walmart.com

### Getting Cookies

1. Log into walmart.com in your browser
2. Go to your orders page
3. Open DevTools (F12) → Network tab
4. Refresh the page
5. Find a 'getOrder' or similar request
6. Right-click → Copy → Copy as cURL
7. Save to a file (e.g., `curl.txt`)
8. Initialize the client: `client.InitializeFromCurl("curl.txt")`

## Example Output

### Basic Example
```
=== Recent Orders ===
Order 200013441152420 - 15 items
Order 200013356814262 - 8 items
...

=== Order Details ===
Order Total: $185.83
Items:
  - Great Value Whole Milk (qty: 1.000)
  - Fresh Banana Fruit (qty: 2.140)
  ...
```

### Ledger Example
```
Order #200013509224581 Payment Details:

Visa ending in 0953:
  Charge 1: $178.96
  Charge 2: $4.12
  Total: $183.08

Walmart Cash: $2.75
```

## Logging

All examples use structured JSON logging to stdout. To disable logging:

```go
config := walmart.ClientConfig{
    Logger: nil,  // Disable logging
}
```

To use text logging instead:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
config := walmart.ClientConfig{
    Logger: logger,
}
```

## Need Help?

- API Documentation: https://pkg.go.dev/github.com/eshaffer321/walmart-client
- Main README: ../README.md
- Issues: https://github.com/eshaffer321/walmart-client-go/issues
