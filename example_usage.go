package walmart

import (
	"fmt"
	"time"
)

// ExampleUsage shows how to use the Walmart client as a library
func ExampleUsage() {
	// Initialize client with default config
	config := ClientConfig{
		RateLimit: 2 * time.Second,
		AutoSave:  true,
	}

	client, err := NewWalmartClient(config)
	if err != nil {
		panic(err)
	}

	// Get recent orders
	orders, err := client.GetRecentOrders(10)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d recent orders\n", len(orders))

	// Get full details for first order
	if len(orders) > 0 {
		order := orders[0]
		isInStore := order.FulfillmentType == "IN_STORE"

		fullOrder, err := client.GetOrder(order.OrderID, isInStore)
		if err != nil {
			panic(err)
		}

		// Access pricing data
		if fullOrder.PriceDetails != nil {
			fmt.Printf("Order Total: %s\n", fullOrder.PriceDetails.GrandTotal.DisplayValue)
		}

		// Access items
		for _, item := range fullOrder.GetItems() {
			if item.ProductInfo != nil {
				fmt.Printf("- %s (qty: %d)\n", item.ProductInfo.Name, item.Quantity)
			}
		}
	}
}
