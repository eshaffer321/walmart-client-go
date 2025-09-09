package walmart

import "fmt"

// OrderResponse is the top-level GraphQL response
type OrderResponse struct {
	Data struct {
		Order *Order `json:"order"`
	} `json:"data"`
}

// Order represents a Walmart order
type Order struct {
	ID             string               `json:"id"`
	Type           string               `json:"type"`
	OrderDate      string               `json:"orderDate"`
	DisplayID      string               `json:"displayId"`
	Title          string               `json:"title"`
	ShortTitle     string               `json:"shortTitle"`
	Groups         []OrderGroup         `json:"groups_2101"`
	Customer       Customer             `json:"customer"`
	Timezone       string               `json:"timezone"`
	PriceDetails   *OrderPriceDetails   `json:"priceDetails"`
	PaymentMethods []OrderPaymentMethod `json:"paymentMethods"`
}

// OrderPriceDetails contains the order-level pricing
type OrderPriceDetails struct {
	SubTotal     *PriceLineItem  `json:"subTotal"`
	TaxTotal     *PriceLineItem  `json:"taxTotal"`
	GrandTotal   *PriceLineItem  `json:"grandTotal"`
	DriverTip    *PriceLineItem  `json:"driverTip"`    // Driver tip for delivery orders
	TotalWithTip *PriceLineItem  `json:"totalWithTip"` // Total including tip (grandTotal + driverTip)
	Savings      *PriceLineItem  `json:"savings"`
	Fees         []PriceLineItem `json:"fees"` // Additional fees including delivery fee
}

// PriceLineItem represents a line item in pricing
type PriceLineItem struct {
	Label        string  `json:"label"`
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

// OrderPaymentMethod represents payment method at order level
type OrderPaymentMethod struct {
	Description string `json:"description"`
	CardType    string `json:"cardType"`
	PaymentType string `json:"paymentType"`
}

// GetItems extracts all items from all groups
func (o *Order) GetItems() []OrderItem {
	var items []OrderItem
	for _, group := range o.Groups {
		items = append(items, group.Items...)
	}
	return items
}

// Customer information
type Customer struct {
	ID                string  `json:"id"`
	FirstName         *string `json:"firstName"`
	LastName          *string `json:"lastName"`
	Email             *string `json:"email"`
	IsGuest           bool    `json:"isGuest"`
	IsEmailRegistered bool    `json:"isEmailRegistered"`
}

// OrderGroup represents a group of items in an order
type OrderGroup struct {
	ID              string          `json:"id"`
	ItemCount       int             `json:"itemCount"`
	Items           []OrderItem     `json:"items"`
	FulfillmentType string          `json:"fulfillmentType"`
	Status          GroupStatus     `json:"status"`
	TotalPrice      *PriceInfo      `json:"totalPrice"`
	Store           *Store          `json:"store"`
	PriceDetails    *PriceDetails   `json:"priceDetails"`
	PaymentDetails  *PaymentDetails `json:"paymentDetails"`
}

// Store represents store information
type Store struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	Address     struct {
		AddressLineOne string `json:"addressLineOne"`
		City           string `json:"city"`
		State          string `json:"state"`
		PostalCode     string `json:"postalCode"`
	} `json:"address"`
}

// PriceDetails contains detailed pricing information
type PriceDetails struct {
	SubTotal     *Money   `json:"subTotal"`
	Tax          *TaxInfo `json:"tax"`
	Savings      *Money   `json:"savings"`
	GrandTotal   *Money   `json:"grandTotal"`
	DriverTip    *Money   `json:"driverTip"`    // Driver tip for delivery orders
	DeliveryFee  *Money   `json:"deliveryFee"`  // Delivery fee
	TotalWithTip *Money   `json:"totalWithTip"` // Total including tip
}

// TaxInfo contains tax information
type TaxInfo struct {
	TaxAmount *Money `json:"taxAmount"`
}

// PaymentDetails contains payment information
type PaymentDetails struct {
	PaymentMethods []PaymentMethod `json:"paymentMethods"`
}

// PaymentMethod represents a payment method
type PaymentMethod struct {
	DisplayName string `json:"displayName"`
	Last4Digits string `json:"last4Digits"`
	Amount      *Money `json:"amount"`
}

// GroupStatus represents the status of an order group
type GroupStatus struct {
	StatusType string  `json:"statusType"`
	Message    Message `json:"message"`
}

// Message structure
type Message struct {
	Parts []MessagePart `json:"parts"`
}

// MessagePart represents a part of a message
type MessagePart struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold"`
	URL       string `json:"url,omitempty"`
	LineBreak bool   `json:"lineBreak"`
}

// OrderItem represents an individual item in an order
type OrderItem struct {
	ID          string       `json:"id"`
	Quantity    float64      `json:"quantity"`
	ProductInfo *ProductInfo `json:"productInfo"`
	PriceInfo   *ItemPrice   `json:"priceInfo"`
}

// ProductInfo contains product details
type ProductInfo struct {
	Name          string    `json:"name"`
	USItemID      string    `json:"usItemId"`
	ImageInfo     ImageInfo `json:"imageInfo"`
	OfferID       string    `json:"offerId"`
	IsAlcohol     bool      `json:"isAlcohol"`
	SalesUnitType string    `json:"salesUnitType"`
}

// ImageInfo contains image URLs
type ImageInfo struct {
	ThumbnailURL string `json:"thumbnailUrl"`
}

// ItemPrice represents pricing for an item
type ItemPrice struct {
	LinePrice *Price `json:"linePrice"`
	UnitPrice *Price `json:"unitPrice"`
}

// Price represents a monetary value
type Price struct {
	DisplayValue string  `json:"displayValue"`
	Value        float64 `json:"value"`
}

// Money is an alias for Price (they have same structure)
type Money = Price

// PriceInfo contains total price information
type PriceInfo struct {
	Total Price `json:"total"`
}

// CalculateOrderTotal calculates the total from all items
func (o *Order) CalculateOrderTotal() float64 {
	total := 0.0
	for _, group := range o.Groups {
		for _, item := range group.Items {
			if item.PriceInfo != nil && item.PriceInfo.LinePrice != nil {
				total += item.PriceInfo.LinePrice.Value
			}
		}
	}
	return total
}

// GetItemCount returns the total number of items
func (o *Order) GetItemCount() int {
	count := 0
	for _, group := range o.Groups {
		count += group.ItemCount
	}
	return count
}

// CalculateTotalWithTip calculates and sets the TotalWithTip field
func (o *Order) CalculateTotalWithTip() {
	if o.PriceDetails == nil || o.PriceDetails.GrandTotal == nil {
		return
	}

	total := o.PriceDetails.GrandTotal.Value
	tipAmount := 0.0

	if o.PriceDetails.DriverTip != nil {
		tipAmount = o.PriceDetails.DriverTip.Value
	}

	if tipAmount > 0 {
		totalWithTip := total + tipAmount
		o.PriceDetails.TotalWithTip = &PriceLineItem{
			Label:        "Total with Tip",
			Value:        totalWithTip,
			DisplayValue: fmt.Sprintf("$%.2f", totalWithTip),
		}
	}
}

// IsDeliveryOrder checks if the order is a delivery order
func (o *Order) IsDeliveryOrder() bool {
	for _, group := range o.Groups {
		if group.FulfillmentType == "SC_DELIVERY" || group.FulfillmentType == "DFS" {
			return true
		}
	}
	return false
}
