package main

import (
	"fmt"
	"log"
	"time"

	walmart "github.com/eshaffer321/walmart-client"
)

func main() {
	// Initialize the client with your saved cookies
	config := walmart.ClientConfig{
		CookieFile: "cookies.json", // Path to your cookie file
		RateLimit:  time.Second,
		AutoSave:   true,
	}

	client, err := walmart.NewWalmartClient(config)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}

	// Example order ID - replace with your actual order ID
	orderID := "200013509224581"

	// Get the order ledger showing actual credit card charges
	ledger, err := client.GetOrderLedger(orderID)
	if err != nil {
		log.Fatal("Failed to get order ledger:", err)
	}

	// Display the ledger information
	fmt.Printf("Order Ledger for Order #%s\n", ledger.OrderID)
	fmt.Println("=" + string(make([]byte, 50)))

	var grandTotal float64
	for _, pm := range ledger.PaymentMethods {
		fmt.Printf("\nPayment Method: %s - %s", pm.CardType, pm.PaymentType)
		if pm.LastFour != "" {
			fmt.Printf(" (ending in %s)", pm.LastFour)
		}
		fmt.Println()

		fmt.Println("Final Charges:")
		for i, charge := range pm.FinalCharges {
			if charge >= 0 {
				fmt.Printf("  Charge %d: $%.2f\n", i+1, charge)
			} else {
				fmt.Printf("  Refund %d: -$%.2f\n", i+1, -charge)
			}
		}

		fmt.Printf("  Total for this card: $%.2f\n", pm.TotalCharged)
		grandTotal += pm.TotalCharged
	}

	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("Grand Total: $%.2f\n", grandTotal)

	// Example usage for matching with bank transactions
	fmt.Println("\n--- Bank Transaction Matching ---")
	fmt.Println("To match with your bank statement, look for:")

	for _, pm := range ledger.PaymentMethods {
		if pm.PaymentType == "CREDITCARD" {
			fmt.Printf("\n%s", pm.CardType)
			if pm.LastFour != "" {
				fmt.Printf(" ending in %s:\n", pm.LastFour)
			} else {
				fmt.Println(":")
			}

			for _, charge := range pm.FinalCharges {
				if charge > 0 {
					fmt.Printf("  - A charge of $%.2f\n", charge)
				}
			}
		}
	}
}