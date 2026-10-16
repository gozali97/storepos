package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"testing"
)

func TestCSVAbove100000RowsPreservesValues(t *testing.T) {
	rows := make([]PaymentRow, 100005)
	for i := range rows {
		rows[i] = PaymentRow{Date: "2026-10-13", Method: "cash", Received: fmt.Sprint(i), Refunded: "0", Net: "-100"}
	}
	var buf bytes.Buffer
	n, err := exportCSV(&buf, rows, nil)
	if err != nil || n != 100005 {
		t.Fatal(n, err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil || len(records) != 100006 {
		t.Fatal(len(records), err)
	}
	if records[100005][5] != "100004" || records[1][7] != "-100" {
		t.Fatal("truncated or altered numeric values")
	}
}

func TestSalesCSVPreservesDeliveryFeeAndNetBillColumns(t *testing.T) {
	fee, bill := "-1000000", "0"
	var buf bytes.Buffer
	if n, err := exportCSV(&buf, []SalesRow{{DeliveryFee: &fee, BillNet: &bill}}, nil); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for i, name := range records[0] {
		got[name] = records[1][i]
	}
	if got["delivery_fee_cents"] != fee || got["net_bill_cents"] != bill {
		t.Fatalf("missing fee/bill values: %v", got)
	}
}
