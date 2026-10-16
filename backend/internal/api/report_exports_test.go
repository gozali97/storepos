package apiserver

import (
	"context"
	"encoding/csv"
	"github.com/storepos/storepos/internal/platform/testdb"
	"github.com/storepos/storepos/internal/reporting"
	"net/http"
	"sync"
	"testing"
)

func TestBackgroundExportClaimDownloadRevocationAndExpiry(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@export.test")
	ctx := context.Background()
	db := pool.Underlying()
	var branch string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@export.test", biz.String())
	code, r := owner.do("POST", "/api/v1/reports/products/exports?branch_id="+branch+"&from=2026-10-13&to=2026-10-13", nil, nil)
	if code != 202 {
		t.Fatal(code, r)
	}
	id := r["export_id"].(string)
	path := "/api/v1/report-exports/" + id
	if code, _ := owner.do("GET", path+"?download=true", nil, nil); code != 409 {
		t.Fatalf("pending download: %d", code)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ran, err := reporting.ProcessExport(ctx, pool)
			if err != nil {
				t.Error(err)
			}
			results <- ran
		}()
	}
	wg.Wait()
	close(results)
	n := 0
	for ran := range results {
		if ran {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("duplicate claim: %d", n)
	}
	code, r = owner.do("GET", path, nil, nil)
	if code != 200 || r["status"] != "completed" {
		t.Fatal(code, r)
	}
	req, err := http.NewRequest("GET", base+path+"?download=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(HeaderBusinessID, biz.String())
	res, err := owner.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(res.Body).ReadAll()
	res.Body.Close()
	if err != nil || len(records) != 1 {
		t.Fatal(records, err)
	}
	if _, err := db.Exec(ctx, `UPDATE memberships SET role='manager' WHERE business_id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if code, _ := owner.do("GET", path+"?download=true", nil, nil); code != 403 {
		t.Fatalf("profit revoke: %d", code)
	}
	if _, err := db.Exec(ctx, `UPDATE memberships SET role='owner' WHERE business_id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE report_exports SET expires_at=now()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := reporting.ProcessExport(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var status string
	var artifact []byte
	if err := db.QueryRow(ctx, `SELECT status,artifact FROM report_exports WHERE id=$1`, id).Scan(&status, &artifact); err != nil {
		t.Fatal(err)
	}
	if status != "expired" || artifact != nil {
		t.Fatal("artifact retained", status)
	}
	if code, _ := owner.do("GET", path, nil, nil); code != 410 {
		t.Fatalf("expiry: %d", code)
	}
}
