package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	walmart "github.com/eshaffer321/walmart-client-go/v2"
)

func main() {
	// Command flags.
	var (
		initCurl   = flag.String("init", "", "Initialize from curl file")
		orderID    = flag.String("order", "", "Fetch specific order ID")
		history    = flag.Bool("history", false, "Show recent purchase history")
		search     = flag.String("search", "", "Search orders for item")
		listAll    = flag.Bool("list-all", false, "List all orders (max 5 pages)")
		status     = flag.Bool("status", false, "Show cookie status")
		refresh    = flag.Bool("refresh", false, "Refresh cookies from browser")
		export     = flag.String("export", "", "Export cookies to file")
		configPath = flag.String("config", "", "Config directory (default: ~/.walmart-api)")
		daemon     = flag.Bool("daemon", false, "Run in daemon mode, auto-refresh cookies")
	)

	flag.Parse()

	// Determine config directory.
	configDir := *configPath
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to determine home directory: %v", err)
		}
		configDir = filepath.Join(homeDir, ".walmart-api")
	}

	// Create client.
	config := walmart.ClientConfig{
		CookieDir: configDir,
		RateLimit: 2 * time.Second,
		AutoSave:  true,
	}

	client, err := walmart.NewWalmartClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Handle commands.
	switch {
	case *initCurl != "":
		// Initialize from curl file.
		if err := client.InitializeFromCurl(*initCurl); err != nil {
			log.Fatalf("Failed to initialize: %v", err)
		}
		fmt.Println("✅ Successfully initialized from curl file")
		client.Status()

	case *status:
		// Show status.
		client.Status()

	case *refresh:
		// Interactive refresh.
		if err := client.RefreshFromBrowser(); err != nil {
			log.Fatalf("Failed to refresh: %v", err)
		}

	case *orderID != "":
		exitOnError(handleOrder(client, *orderID))

	case *history:
		exitOnError(handleHistory(client))

	case *search != "":
		exitOnError(handleSearch(client, *search))

	case *listAll:
		exitOnError(handleListAll(client))

	case *export != "":
		handleExport(client, *export)

	case *daemon:
		// Run in daemon mode.
		runDaemon(client)

	default:
		printUsage(client, configDir)
	}
}

// exitOnError terminates the program with a non-zero status when err is non-nil.
// Handlers print their own diagnostics, so this only needs to set the exit code.
func exitOnError(err error) {
	if err != nil {
		os.Exit(1)
	}
}

// handleOrder fetches and displays a single order.
func handleOrder(client *walmart.WalmartClient, orderID string) error {
	fmt.Printf("\n📦 Fetching order %s...\n", orderID)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	order, err := client.GetOrderAutoDetect(ctx, orderID)
	if err != nil {
		// Try to help user recover.
		fmt.Printf("\n❌ Failed: %v\n", err)
		fmt.Println("\nTroubleshooting:")
		fmt.Println("1. Your cookies might be expired")
		fmt.Println("2. Try: ./walmart -refresh")
		fmt.Println("3. Or: ./walmart -init curl.txt")
		return err
	}
	displayOrder(order)
	return nil
}

// handleHistory fetches and displays recent purchase history.
func handleHistory(client *walmart.WalmartClient) error {
	fmt.Println("\n📋 Fetching recent orders...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	orders, err := client.GetRecentOrders(ctx, 10)
	if err != nil {
		fmt.Printf("\n❌ Failed: %v\n", err)
		return err
	}
	displayOrderHistory(orders)
	return nil
}

// handleSearch searches orders for a term and displays the results.
func handleSearch(client *walmart.WalmartClient, term string) error {
	fmt.Printf("\n🔍 Searching for '%s' in orders...\n", term)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	orders, err := client.SearchOrders(ctx, term, 20)
	if err != nil {
		fmt.Printf("\n❌ Failed: %v\n", err)
		return err
	}
	displayOrderHistory(orders)
	return nil
}

// handleListAll fetches all orders with pagination and displays them.
func handleListAll(client *walmart.WalmartClient) error {
	fmt.Println("\n📋 Fetching all orders (max 5 pages)...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	orders, err := client.GetAllOrders(ctx, 5)
	if err != nil {
		fmt.Printf("\n❌ Failed: %v\n", err)
		return err
	}
	fmt.Printf("\nFetched %d total orders\n", len(orders))
	displayOrderHistory(orders)
	return nil
}

// handleExport writes the current cookies to the given path, exiting on failure.
func handleExport(client *walmart.WalmartClient, path string) {
	if err := client.SaveCookies(); err != nil {
		log.Fatalf("Failed to export: %v", err)
	}

	// Also create a simple format.
	simpleCookies := client.ExportCookies()

	data, err := json.MarshalIndent(simpleCookies, "", "  ")
	if err != nil {
		log.Fatalf("Failed to encode cookies: %v", err)
	}
	// Cookie exports contain session credentials, so restrict to owner read/write.
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Fatalf("Failed to write export file: %v", err)
	}
	fmt.Printf("✅ Exported cookies to %s\n", path)
}

// printUsage prints CLI usage information and the current cookie status.
func printUsage(client *walmart.WalmartClient, configDir string) {
	fmt.Println("Walmart API Client")
	fmt.Println("\nUsage:")
	fmt.Println("  First time setup:")
	fmt.Println("    ./walmart -init curl.txt     # Initialize from curl file")
	fmt.Println("")
	fmt.Println("  Fetch orders:")
	fmt.Println("    ./walmart -order ORDER_ID    # Fetch specific order")
	fmt.Println("    ./walmart -history           # Show recent orders")
	fmt.Println("    ./walmart -search cheese     # Search orders")
	fmt.Println("    ./walmart -list-all          # List all orders")
	fmt.Println("")
	fmt.Println("  Maintenance:")
	fmt.Println("    ./walmart -status            # Show cookie status")
	fmt.Println("    ./walmart -refresh           # Refresh cookies from browser")
	fmt.Println("    ./walmart -export cookies.json # Export cookies")
	fmt.Println("")
	fmt.Println("Cookie storage:")
	fmt.Printf("  %s\n", filepath.Join(configDir, "cookies.json"))

	// Check if initialized.
	if client.CookieCount() == 0 {
		fmt.Println("\n⚠️  No cookies found. Run: ./walmart -init curl.txt")
	} else {
		fmt.Printf("\n✅ %d cookies loaded\n", client.CookieCount())
	}
}

func displayOrderHistory(orders []walmart.OrderSummary) {
	fmt.Printf("\n=== Order History (%d orders) ===\n", len(orders))

	if len(orders) == 0 {
		fmt.Println("No orders found")
		return
	}

	for i, order := range orders {
		fmt.Printf("\n%d. Order #%s\n", i+1, order.OrderID)

		// Show type and status.
		fmt.Printf("   Type: %s", order.FulfillmentType)
		if order.Status != nil {
			fmt.Printf(" | Status: %s", order.Status.StatusType)
		}
		fmt.Println()

		// Show date if available.
		if order.Status != nil && len(order.Status.Message.Parts) > 0 {
			fmt.Printf("   Date: %s\n", order.Status.Message.Parts[0].Text)
		}

		// Show store or delivery info.
		if order.Store != nil {
			fmt.Printf("   Store: %s\n", order.Store.Name)
		} else {
			fmt.Printf("   Delivery: %s\n", order.DeliveryMessage)
		}

		// Show items.
		fmt.Printf("   Items (%d):\n", order.ItemCount)
		for j, item := range order.Items {
			if j >= 3 {
				fmt.Printf("     ... and %d more\n", order.ItemCount-3)
				break
			}
			fmt.Printf("     - %s (qty: %d)\n", item.Name, item.Quantity)
		}

		// Show if it's in-store vs delivery for fetching.
		if order.FulfillmentType == "IN_STORE" {
			fmt.Printf("   📍 In-store purchase\n")
		} else {
			fmt.Printf("   🚚 Delivery order\n")
		}
	}

	fmt.Println("\nTo fetch full details of an order:")
	fmt.Println("  ./walmart -order ORDER_ID")
}

func displayOrder(order *walmart.Order) {
	fmt.Println("\n=== Order Details ===")
	fmt.Printf("Order ID:     %s\n", order.ID)
	fmt.Printf("Display ID:   %s\n", order.DisplayID)

	// Parse and format date.
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", order.OrderDate); err == nil {
		fmt.Printf("Date:         %s\n", t.Format("Jan 2, 2006 at 3:04 PM"))
	} else {
		fmt.Printf("Date:         %s\n", order.OrderDate)
	}

	// Show store.
	if len(order.Groups) > 0 && order.Groups[0].Store != nil {
		store := order.Groups[0].Store
		fmt.Printf("Store:        %s\n", store.DisplayName)
	}

	// Items.
	items := order.GetItems()
	fmt.Printf("\nItems (%d):\n", len(items))

	var subtotal float64
	for i, item := range items {
		if item.ProductInfo != nil {
			fmt.Printf("\n  %d. %s\n", i+1, item.ProductInfo.Name)
			fmt.Printf("     Item #%s\n", item.ProductInfo.USItemID)
			fmt.Printf("     Qty: %.3f", item.Quantity)

			if item.PriceInfo != nil && item.PriceInfo.LinePrice != nil {
				if item.PriceInfo.UnitPrice != nil {
					fmt.Printf(" × %s = %s\n",
						item.PriceInfo.UnitPrice.DisplayValue,
						item.PriceInfo.LinePrice.DisplayValue)
				} else {
					fmt.Printf(" = %s\n", item.PriceInfo.LinePrice.DisplayValue)
				}
				subtotal += item.PriceInfo.LinePrice.Value
			} else {
				fmt.Println()
			}
		}
	}

	// Price summary - now at order level.
	if order.PriceDetails != nil {
		fmt.Println("\n=== Price Summary ===")

		if order.PriceDetails.SubTotal != nil {
			fmt.Printf("Subtotal:     %s\n", order.PriceDetails.SubTotal.DisplayValue)
		}

		if order.PriceDetails.TaxTotal != nil {
			fmt.Printf("Tax:          %s\n", order.PriceDetails.TaxTotal.DisplayValue)
		}

		if order.PriceDetails.Savings != nil && order.PriceDetails.Savings.Value > 0 {
			fmt.Printf("Savings:      -%s\n", order.PriceDetails.Savings.DisplayValue)
		}

		if order.PriceDetails.GrandTotal != nil {
			fmt.Printf("Total:        %s\n", order.PriceDetails.GrandTotal.DisplayValue)
		}
	}

	// Payment methods - now at order level.
	if len(order.PaymentMethods) > 0 {
		fmt.Println("\n=== Payment ===")
		for _, payment := range order.PaymentMethods {
			fmt.Println(payment.Description)
		}
	}

	fmt.Println("\n✅ Order retrieved successfully")
}

func runDaemon(client *walmart.WalmartClient) {
	fmt.Println("🔄 Running in daemon mode...")
	fmt.Println("   Will auto-refresh cookies every 30 minutes")
	fmt.Println("   Press Ctrl+C to stop")

	// Set up graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	// Test order ID for health checks.
	testOrderID := "18420337004257359578"

	for {
		select {
		case <-sigCh:
			fmt.Println("\n👋 Shutting down daemon...")
			return
		case <-ticker.C:
			fmt.Printf("\n[%s] Checking cookie health...\n", time.Now().Format("15:04:05"))

			// Try to fetch an order as health check.
			checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
			_, err := client.GetOrder(checkCtx, testOrderID, true)
			checkCancel()
			if err != nil {
				fmt.Printf("⚠️  Cookies might be stale: %v\n", err)
				fmt.Println("   Consider running: ./walmart -refresh")
			} else {
				fmt.Println("✅ Cookies are healthy")
			}

			// Show status.
			client.Status()
		}
	}
}
