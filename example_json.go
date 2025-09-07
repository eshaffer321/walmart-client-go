package walmart

import (
	"encoding/json"
	"fmt"
	"log"
)

// ExampleJSON shows how to get data as JSON
func ExampleJSON() {
	client, err := NewWalmartClient(ClientConfig{})
	if err != nil {
		log.Fatal(err)
	}

	// Get orders
	orders, err := client.GetRecentOrders(5)
	if err != nil {
		panic(err)
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("Orders as JSON:")
	fmt.Println(string(jsonData))

	// Get single order
	if len(orders) > 0 {
		order, err := client.GetOrder(orders[0].OrderID, true)
		if err != nil {
			panic(err)
		}

		// Access as struct
		fmt.Printf("Order Total (as float): %.2f\n", order.PriceDetails.GrandTotal.Value)
		fmt.Printf("Order Total (as string): %s\n", order.PriceDetails.GrandTotal.DisplayValue)

		// Or convert whole order to JSON
		orderJSON, err := json.MarshalIndent(order, "", "  ")
		if err != nil {
			panic(err)
		}
		fmt.Println("\nFull order as JSON:")
		fmt.Println(string(orderJSON))
	}
}

// GetOrdersAsJSON is a helper that returns orders directly as JSON string
func (c *WalmartClient) GetOrdersAsJSON(limit int) (string, error) {
	orders, err := c.GetRecentOrders(limit)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GetOrderAsJSON returns a single order as JSON string
func (c *WalmartClient) GetOrderAsJSON(orderID string, isInStore bool) (string, error) {
	order, err := c.GetOrder(orderID, isInStore)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
