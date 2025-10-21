package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	walmart "github.com/eshaffer321/walmart-client"
)

func main() {
	// Create a structured logger (JSON format to stdout)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Initialize client with logger
	// To disable logging, pass nil instead of logger
	config := walmart.ClientConfig{
		RateLimit: 2 * time.Second,
		AutoSave:  true,
		Logger:    logger, // Pass nil to disable all logging
	}

	client, err := walmart.NewWalmartClient(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create client: %v", err))
	}

	// Example 1: Get recent orders
	fmt.Println("\n=== Recent Orders ===")
	orders, err := client.GetRecentOrders(5)
	if err != nil {
		panic(fmt.Sprintf("Failed to get recent orders: %v", err))
	}

	for _, order := range orders {
		fmt.Printf("Order %s - %d items\n", order.OrderID, order.ItemCount)
	}

	// Example 2: Get specific order with full details
	if len(orders) > 0 {
		fmt.Println("\n=== Order Details ===")
		orderID := orders[0].OrderID
		isInStore := orders[0].FulfillmentType == "IN_STORE"

		fullOrder, err := client.GetOrder(orderID, isInStore)
		if err != nil {
			panic(fmt.Sprintf("Failed to get order: %v", err))
		}

		fmt.Printf("Order Total: %s\n", fullOrder.PriceDetails.GrandTotal.DisplayValue)
		fmt.Printf("Items:\n")
		for _, item := range fullOrder.GetItems() {
			if item.ProductInfo != nil {
				fmt.Printf("  - %s (qty: %.3f)\n", item.ProductInfo.Name, item.Quantity)
			}
		}
	}

	// Example 3: Search orders
	fmt.Println("\n=== Search Results ===")
	results, err := client.SearchOrders("bread", 10)
	if err != nil {
		panic(fmt.Sprintf("Failed to search orders: %v", err))
	}

	fmt.Printf("Found %d orders containing 'bread'\n", len(results))

	fmt.Println("\n=== Logger Note ===")
	fmt.Println("All operations above are logged with structured JSON output to stdout.")
	fmt.Println("To disable logging, set Logger: nil in ClientConfig.")
}
