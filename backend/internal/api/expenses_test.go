package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestExpensesOutsideCashAndShiftAtomicReplay(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@expense.test")
	db := pool.Underlying()
	ctx := context.Background()
	var branch, reg string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K1') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	c := login(t, newServer(t, pool), "owner@expense.test", biz.String())
	body := map[string]string{"branch_id": branch, "expense_date": "2026-10-13", "category": "Listrik", "amount_cents": "30000000", "method": "outside_cash"}
	for i := 0; i < 2; i++ {
		if code, r := c.do("POST", "/api/v1/expenses", body, map[string]string{"Idempotency-Key": "outside"}); code != 201 {
			t.Fatal(code, r)
		}
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM cash_movements`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("outside changed cash: %d %v", n, err)
	}
	body["method"] = "shift_cash"
	body["register_id"] = reg
	if code, _ := c.do("POST", "/api/v1/expenses", body, map[string]string{"Idempotency-Key": "cash"}); code != 422 {
		t.Fatalf("closed shift: %d", code)
	}
	if code, r := c.do("POST", "/api/v1/shifts/open", map[string]string{"branch_id": branch, "register_id": reg, "opening_cash_cents": "50000000"}, nil); code != 201 {
		t.Fatal(code, r)
	}
	for i := 0; i < 2; i++ {
		if code, r := c.do("POST", "/api/v1/expenses", body, map[string]string{"Idempotency-Key": "cash"}); code != 201 {
			t.Fatal(code, r)
		}
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM expenses`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("expenses: %d %v", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM cash_movements`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("duplicate cash out: %d %v", n, err)
	}
	code, r := c.do("POST", "/api/v1/shifts/close", map[string]string{"branch_id": branch, "register_id": reg, "counted_cash_cents": "20000000"}, nil)
	if code != 200 || r["expected_cash_cents"] != "20000000" {
		t.Fatal(code, r)
	}
	shiftID, ok := r["shift_id"].(string)
	if !ok || shiftID == "" {
		t.Fatalf("close response missing shift ID: %v", r)
	}
	if summaryCode, summary := c.do("GET", "/api/v1/shifts/"+shiftID+"/summary", nil, nil); summaryCode != 200 || summary["shift_id"] != shiftID || summary["expected_cash_cents"] != "20000000" {
		t.Fatalf("closed shift summary: %d %v", summaryCode, summary)
	}
	code, r = c.do("GET", "/api/v1/expenses?branch_id="+branch+"&from=2026-10-13&to=2026-10-13", nil, nil)
	if code != 200 || len(r["expenses"].([]any)) != 2 {
		t.Fatal(code, r)
	}
}
