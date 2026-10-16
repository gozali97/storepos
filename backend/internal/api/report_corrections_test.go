package apiserver

import (
	"context"
	"encoding/csv"
	"github.com/storepos/storepos/internal/platform/testdb"
	"net/http"
	"testing"
)

func TestCorrectionsCSVAndPermission(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "A", "owner@correct.test")
	seedUser(t, pool, "cashier@correct.test", "cashier", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var branch, reg, shift string
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	var uid string
	if err := db.QueryRow(ctx, `SELECT id::text FROM users WHERE email='owner@correct.test'`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO shifts(id,business_id,branch_id,register_id,opened_by,opening_cash_cents) VALUES(gen_random_uuid(),$1,$2,$3,$4,0) RETURNING id::text`, biz, branch, reg, uid).Scan(&shift); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO sales(id,business_id,branch_id,register_id,shift_id,receipt_number,subtotal_gross_cents,line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by,discount_reason,completed_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,'R1',1100,100,0,0,0,0,1000,0,$5,'=HYPERLINK("bad")','2026-10-13T01:00:00Z')`, biz, branch, reg, shift, uid)
	base := newServer(t, pool)
	owner := login(t, base, "owner@correct.test", biz.String())
	cashier := login(t, base, "cashier@correct.test", biz.String())
	path := "/api/v1/reports/corrections?branch_id=" + branch + "&from=2026-10-13&to=2026-10-13"
	code, r := owner.do("GET", path, nil, nil)
	if code != 200 || len(r["rows"].([]any)) != 1 {
		t.Fatal(code, r)
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
		t.Fatalf("csv status: %d", res.StatusCode)
	}
	records, err := csv.NewReader(res.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][9] != "100" || records[1][11] != "100" || records[1][8][0] != '\'' {
		t.Fatal(records)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type='corrections_report' AND action='report.exported'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit: %d %v", n, err)
	}
}
