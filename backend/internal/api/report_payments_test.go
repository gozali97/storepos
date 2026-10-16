package apiserver

import (
	"context"
	"encoding/csv"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"net/http"
	"testing"
)

func TestPaymentReportCSVMatchesJSONAndAudit(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "A", "owner@report.test")
	seedUser(t, pool, "cashier@report.test", "cashier", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var branch, reg, shift string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	var uid idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM users WHERE email='owner@report.test'`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO shifts(id,business_id,branch_id,register_id,opened_by,opening_cash_cents) VALUES(gen_random_uuid(),$1,$2,$3,$4,0) RETURNING id::text`, biz, branch, reg, uid).Scan(&shift); err != nil {
		t.Fatal(err)
	}
	sale := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO sales(id,business_id,branch_id,register_id,shift_id,receipt_number,subtotal_gross_cents,line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by) VALUES($1,$2,$3,$4,$5,'R1',100,0,0,0,0,0,100,0,$6)`, sale, biz, branch, reg, shift, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO payments(id,business_id,sale_id,method,amount_cents,tendered_cents,created_at) VALUES($1,$2,$3,'cash',100,100,'2026-10-13T01:00:00Z')`, idkit.New(), biz, sale); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@report.test", biz.String())
	cashier := login(t, base, "cashier@report.test", biz.String())
	path := "/api/v1/reports/payments?branch_id=" + branch + "&from=2026-10-13&to=2026-10-13"
	code, json := owner.do("GET", path, nil, nil)
	if code != 200 || len(json["rows"].([]any)) != 1 {
		t.Fatal(code, json)
	}
	if code, _ := cashier.do("GET", path, nil, nil); code != 403 {
		t.Fatalf("permission: %d", code)
	}
	req, err := http.NewRequest("GET", base+path+"&format=csv", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(HeaderBusinessID, biz.String())
	res, err := owner.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("CSV: %d", res.StatusCode)
	}
	records, err := csv.NewReader(res.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	row := json["rows"].([]any)[0].(map[string]any)
	if len(records) != 2 || records[1][5] != row["received_cents"] || records[1][6] != row["refunded_cents"] || records[1][7] != row["net_cents"] {
		t.Fatal(records, row)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='report.exported'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit: %d %v", n, err)
	}
}
