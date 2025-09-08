package main

import (
	"fmt"
	"log"
	"time"

	walmart "github.com/eshaffer321/walmart-client"
)

func main() {
	// Initialize client
	config := walmart.ClientConfig{
		RateLimit: 2 * time.Second,
		AutoSave:  true,
	}

	client, err := walmart.NewWalmartClient(config)
	if err != nil {
		log.Fatal(err)
	}

	// Example 1: Get recent orders
	fmt.Println("=== Recent Orders ===")
	orders, err := client.GetRecentOrders(5)
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
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
		log.Fatal(err)
	}

	fmt.Printf("Found %d orders containing 'bread'\n", len(results))
}
