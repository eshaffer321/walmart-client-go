# Walmart API Client

A robust Go client for accessing Walmart order history and purchase data through their GraphQL API.

## Features

- 🛒 Fetch complete order history (both in-store and delivery orders)
- 📦 Get detailed order information including items, prices, tax, and totals
- 🔍 Search orders for specific items
- 📄 Pagination support for large order histories
- 🍪 Automatic cookie management with rotation to prevent staleness
- 💾 Persistent cookie storage in `~/.walmart-api/cookies.json`

## Installation

```bash
go build -o walmart .
```

## Setup

1. **Get your cookies from Walmart.com:**
   - Log into walmart.com in Chrome/Firefox
   - Go to your orders page
   - Open DevTools (F12) → Network tab
   - Refresh the page
   - Find any 'getOrder' request
   - Right-click → Copy → Copy as cURL
   - Save to a file (e.g., `curl.txt`)

2. **Initialize the client:**
```bash
./walmart -init curl.txt
```

This saves your cookies to `~/.walmart-api/cookies.json` for future use.

## Usage

### View Recent Orders
```bash
./walmart -history

# Output:
# === Order History (10 orders) ===
# 
# 1. Order #18420337004257359578
#    Type: IN_STORE | Status: IN_STORE
#    Date: Sep 05, 2025 purchase
#    Store: MERIDIAN Supercenter
#    Items (3):
#      - Great Value Cracker Cut Sliced 4 Cheese Tray, 16 oz (qty: 1)
#      ...
```

### Search Orders
```bash
./walmart -search "cheese"
```

### Get Order Details
```bash
./walmart -order 18420337004257359578

# Output:
# === Order Details ===
# Order ID:     18420337004257359578
# Display ID:   1842-0337-0042-5735-9578
# Date:         Sep 5, 2025 at 4:16 PM
# 
# Items (3):
#   1. Great Value Cracker Cut Sliced 4 Cheese Tray, 16 oz
#      Item #814783251
#      Qty: 1 = $4.98
#   ...
# 
# === Price Summary ===
# Subtotal:     $7.14
# Tax:          $0.43
# Total:        $7.57
# 
# === Payment ===
# Visa ending in 0953
```

### List All Orders (with pagination)
```bash
./walmart -list-all
```

### Check Cookie Status
```bash
./walmart -status

# Output:
# === Cookie Store Status ===
# Total cookies: 61
# Cookie file: /Users/you/.walmart-api/cookies.json
# Essential cookies: 6
# 
# Essential cookies:
#   ✅ CID: 2m30s ago
#   ✅ SPID: 2m30s ago
#   ✅ auth: 2m30s ago
#   ✅ customer: 2m30s ago
```

### Refresh Cookies
```bash
./walmart -refresh
# Follow prompts to update cookies from browser
```

## How It Works

### Authentication
- Uses 61 cookies from your browser session
- **CID** and **SPID** are the essential auth cookies
- However, ALL 61 cookies are required to avoid bot detection (429/418 errors)
- 19 cookies automatically update with each request to prevent staleness

### API Endpoints

1. **Purchase History** (`PurchaseHistoryV2`)
   - Hash: `2c3d5a832b56671dca1ed0ec84940f274d0bc80821db4ad7481e496c0ad5847e`
   - Returns all order types (IN_STORE, DELIVERY, PICKUP)
   - Supports pagination with cursor
   - Filtering by date range, order type, search terms

2. **Order Details** (`getOrder`)
   - Hash: `d0622497daef19150438d07c506739d451cad6749cf45c3b4db95f2f5a0a65c4`
   - Returns complete order information
   - Automatically detects if order is IN_STORE or DELIVERY

### Data Available

Each order includes:
- ✅ Order ID, date, and display ID
- ✅ Complete item list with names, quantities, and item IDs
- ✅ Individual item prices
- ✅ Subtotal, tax, and total amounts
- ✅ Payment method information
- ✅ Store information (for in-store purchases)
- ✅ Delivery details (for online orders)

## File Structure

```
walmart-client/
├── main.go              # CLI interface
├── client.go            # Main client with cookie management
├── models.go            # Data structures for orders
├── purchase_history.go  # Purchase history API methods
└── analysis/            # Test and analysis scripts (can be deleted)
```

## Cookie Storage

Cookies are stored in `~/.walmart-api/cookies.json` with metadata:
```json
{
  "cookies": {
    "CID": {
      "value": "...",
      "last_update": "2025-09-07T08:40:57Z",
      "source": "curl",
      "essential": true
    },
    ...
  },
  "last_update": "2025-09-07T08:40:57Z"
}
```

## Technical Details

### Rate Limiting
- Built-in 2-second delay between requests
- Automatic cookie updates to prevent staleness
- Proper error handling for rate limits (429) and bot detection (418)

### GraphQL Persisted Queries
Walmart uses persisted queries where the query is stored server-side and referenced by hash:
- Each operation type has a unique hash
- Client sends hash + variables instead of full query
- Reduces bandwidth and hides query complexity

### Order Types
- **IN_STORE**: Physical store purchases (`orderIsInStore: true`)
- **DELIVERY**: Online orders delivered to home (`orderIsInStore: false`)
- **PICKUP**: Online orders picked up at store

## Notes

- This is for personal use only - be respectful of Walmart's servers
- Cookies expire after some time - refresh from browser when needed
- Rate limiting is enforced to avoid detection
- All 61 cookies are required despite only 2 containing auth data

## License

For personal use only. This tool is designed for accessing your own order history.