package walmart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	lineTypeOrderCharge = "ORDER_CHARGE"
	lineTypeDriverTip   = "ADDITIONAL_DRIVER_TIP"
)

// A delivery order whose driver tip is charged on its own, a day after the
// order charges, with temporary holds that must not appear as charges.
const ledgerWithTipResponse = `{
	"data": {
		"getOrderLedger": {
			"paymentMethodsLedgers": [
				{
					"paymentType": "CREDITCARD",
					"cardType": "VISA",
					"description": "Ending in 1234",
					"transactions": [
						{
							"chargeType": "FINAL_CHARGES",
							"transactionLines": [
								{
									"date": "Nov 05, 2026",
									"rowLines": [
										{"lineType": "ADDITIONAL_DRIVER_TIP", "displayValues": ["$4.00"], "time": "2:01 PM"}
									]
								},
								{
									"date": "Nov 04, 2026",
									"rowLines": [
										{"lineType": "ORDER_CHARGE", "displayValues": ["$39.96"], "time": "2:29 PM"},
										{"lineType": "ORDER_CHARGE", "displayValues": ["$15.95"], "time": "2:01 PM"}
									]
								}
							]
						},
						{
							"chargeType": "TEMPORARY_HOLDS",
							"transactionLines": [
								{
									"date": "Nov 04, 2026",
									"rowLines": [
										{"lineType": "ORDER_ADJUSTMENT_REFUND", "displayValues": ["-$9.25"], "time": "5:24 PM"},
										{"lineType": "TEMPORARY_HOLD", "displayValues": ["$108.58"], "time": "8:02 AM"}
									]
								}
							]
						}
					]
				}
			]
		}
	}
}`

func TestGetOrderLedgerReturnsLineTypesParallelToCharges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ledgerWithTipResponse))
	}))
	defer server.Close()

	client, err := NewWalmartClient(ClientConfig{RateLimit: time.Millisecond, AutoSave: false})
	require.NoError(t, err)
	client.httpClient = &http.Client{Transport: &testTransport{serverURL: server.URL}}

	ledger, err := client.GetOrderLedger(context.Background(), "200015669005518")
	require.NoError(t, err)
	require.Len(t, ledger.PaymentMethods, 1)

	pm := ledger.PaymentMethods[0]
	assert.Equal(t, []float64{4.00, 39.96, 15.95}, pm.FinalCharges)
	assert.Equal(t, []string{lineTypeDriverTip, lineTypeOrderCharge, lineTypeOrderCharge}, pm.LineTypes)
	assert.Len(t, pm.ChargedDates, len(pm.FinalCharges))
}
