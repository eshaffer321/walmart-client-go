package walmart

import (
	"encoding/json"
	"fmt"
)

// ExampleJSON shows how to get data as JSON
func ExampleJSON() {
	client, _ := NewWalmartClient(ClientConfig{})
	
	// Get orders
	orders, err := client.GetRecentOrders(5)
	if err != nil {
		panic(err)
	}
	
	// Convert to JSON
	jsonData, _ := json.MarshalIndent(orders, "", "  ")
	fmt.Println("Orders as JSON:")
	fmt.Println(string(jsonData))
	
	// Get single order
	if len(orders) > 0 {
		order, _ := client.GetOrder(orders[0].OrderID, true)
		
		// Access as struct
		fmt.Printf("Order Total (as float): %.2f\n", order.PriceDetails.GrandTotal.Value)
		fmt.Printf("Order Total (as string): %s\n", order.PriceDetails.GrandTotal.DisplayValue)
		
		// Or convert whole order to JSON
		orderJSON, _ := json.MarshalIndent(order, "", "  ")
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