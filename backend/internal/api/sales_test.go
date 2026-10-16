package apiserver

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestFinalizeIdempotencyOverHTTP(t *testing.T) {
	pool := testdb.New(t)
	bizA := seedBusiness(t, pool, "Toko A", "owner@a.test")
	seedUser(t, pool, "kasir@a.test", "cashier", bizA)
	base := newServer(t, pool)
	ctx := context.Background()
	db := pool.Underlying()

	var branch, reg string
	_ = db.QueryRow(ctx, `INSERT INTO branches (id,business_id,code,name) VALUES (gen_random_uuid(),$1,'JKT','Pusat') RETURNING id::text`, bizA).Scan(&branch)
	_ = db.QueryRow(ctx, `INSERT INTO registers (id,business_id,branch_id,name) VALUES (gen_random_uuid(),$1,$2,'K1') RETURNING id::text`, bizA, branch).Scan(&reg)

	// Kasir hanya boleh bertransaksi di cabang yang ditugaskan (CORE-005).
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches (membership_id, business_id, branch_id)
		SELECT m.id, m.business_id, $2::uuid FROM memberships m JOIN users u ON u.id=m.user_id
		WHERE u.email='kasir@a.test' AND m.business_id=$1`, bizA, branch); err != nil {
		t.Fatal(err)
	}

	owner := login(t, base, "owner@a.test", bizA.String())
	cashier := login(t, base, "kasir@a.test", bizA.String())

	_, res := owner.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]any{{"sku": "K-1", "price_cents": "1500000"}}}, nil)
	vid := res["variants"].([]any)[0].(map[string]any)["id"].(string)

	if code, r := owner.do("POST", "/api/v1/stock/receipts", map[string]any{"branch_id": branch,
		"lines": []map[string]string{{"variant_id": vid, "qty_milli": "10000"}}}, nil); code != 201 {
		t.Fatalf("terima stok: %d %v", code, r)
	}
	sale := map[string]any{"branch_id": branch, "register_id": reg,
		"note":     "Catatan draft",
		"lines":    []map[string]string{{"variant_id": vid, "qty_milli": "1000", "note": "Catatan baris"}},
		"payments": []map[string]string{{"method": "cash", "amount_cents": "2000000"}}}
	key := func(k string) map[string]string { return map[string]string{"Idempotency-Key": k} }

	// Tanpa shift → 422 SHIFT_NOT_OPEN sesuai CORE-017. Tanpa Idempotency-Key → 422.
	if code, r := cashier.do("POST", "/api/v1/sales/finalize", sale, key("k0")); code != 422 || r["error"].(map[string]any)["code"] != "SHIFT_NOT_OPEN" {
		t.Fatalf("harus 422 SHIFT_NOT_OPEN, dapat %d %v", code, r)
	}
	if code, _ := cashier.do("POST", "/api/v1/shifts/open", map[string]any{"branch_id": branch, "register_id": reg, "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatalf("buka shift harus 201, dapat %d", code)
	}
	if code, _ := cashier.do("POST", "/api/v1/sales/finalize", sale, nil); code != 422 {
		t.Fatalf("tanpa kunci harus 422, dapat %d", code)
	}

	code, draft := cashier.do("POST", "/api/v1/drafts", map[string]any{"branch_id": branch, "register_id": reg,
		"cart": map[string]any{"lines": sale["lines"], "note": sale["note"]}}, nil)
	if code != 201 {
		t.Fatalf("draft: %d %v", code, draft)
	}
	sale["draft_id"], sale["draft_version"] = draft["id"], 1
	sale["note"] = "Catatan berbeda"
	if code, _ := cashier.do("POST", "/api/v1/sales/finalize", sale, key("mismatched-note")); code != 409 {
		t.Fatalf("draft note mismatch: %d", code)
	}
	sale["note"] = "Catatan draft"
	// Kunci sama → respons sama, hanya satu transaksi; replay tetap berhasil setelah draft completed.
	c1, r1 := cashier.do("POST", "/api/v1/sales/finalize", sale, key("k1"))
	c2, r2 := cashier.do("POST", "/api/v1/sales/finalize", sale, key("k1"))
	if c1 != 201 || c2 != 201 || r1["sale_id"] != r2["sale_id"] || r1["receipt_number"] != r2["receipt_number"] {
		t.Fatalf("replay harus identik: %d %v / %d %v", c1, r1, c2, r2)
	}
	if code, detail := cashier.do("GET", "/api/v1/sales/"+r1["sale_id"].(string), nil, nil); code != 200 || detail["lines"].([]any)[0].(map[string]any)["remaining_qty_milli"] != "1000" || detail["lines"].([]any)[0].(map[string]any)["cost_cents"] != nil {
		t.Fatalf("sale detail: %d %v", code, detail)
	}
	if code, found := cashier.do("GET", "/api/v1/sales?branch_id="+branch+"&receipt_number="+r1["receipt_number"].(string), nil, nil); code != 200 || len(found["sales"].([]any)) != 1 {
		t.Fatalf("receipt lookup: %d %v", code, found)
	}
	if code, reprint := cashier.do("POST", "/api/v1/sales/"+r1["sale_id"].(string)+"/reprints", nil, nil); code != 201 || reprint["label"] != "SALINAN" {
		t.Fatal(code, reprint)
	}
	var reprintEvents int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='receipt.reprint_requested' AND entity_id=$1`, r1["sale_id"]).Scan(&reprintEvents); err != nil || reprintEvents != 1 {
		t.Fatalf("reprint audit: %d %v", reprintEvents, err)
	}
	linkPath := "/api/v1/sales/" + r1["sale_id"].(string) + "/receipt-links"
	code, link := cashier.do("POST", linkPath, nil, nil)
	if code != 201 {
		t.Fatal(code, link)
	}
	token := link["token"].(string)
	public := &client{t: t, base: base, hc: &http.Client{}}
	if code, receipt := public.do("POST", "/api/v1/public/receipt", map[string]string{"token": token}, nil); code != 200 || receipt["receipt_number"] != r1["receipt_number"] {
		t.Fatal(code, receipt)
	}
	if code, _ := public.do("POST", "/api/v1/public/receipt", map[string]string{"token": token[:len(token)-1] + "!"}, nil); code != 404 {
		t.Fatalf("invalid bearer: %d", code)
	}
	if code, _ := cashier.do("DELETE", linkPath, nil, nil); code != 403 {
		t.Fatalf("cashier revoke: %d", code)
	}
	if code, _ := owner.do("DELETE", linkPath, nil, nil); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if code, _ := public.do("POST", "/api/v1/public/receipt", map[string]string{"token": token}, nil); code != 404 {
		t.Fatalf("revoked receipt: %d", code)
	}
	// Kunci sama, isi berbeda → 409 IDEMPOTENCY_CONFLICT.
	other := map[string]any{"branch_id": branch, "register_id": reg,
		"lines":    []map[string]string{{"variant_id": vid, "qty_milli": "2000"}},
		"payments": []map[string]string{{"method": "cash", "amount_cents": "5000000"}}}
	if code, r := cashier.do("POST", "/api/v1/sales/finalize", other, key("k1")); code != 409 || r["error"].(map[string]any)["code"] != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("isi beda harus IDEMPOTENCY_CONFLICT, dapat %d %v", code, r)
	}

	if code, _ := cashier.do("POST", "/api/v1/sales/finalize", sale, key("duplicate-draft")); code != 409 {
		t.Fatalf("draft completed tidak boleh dijual dua kali: %d", code)
	}
	var draftStatus string
	if err := db.QueryRow(ctx, `SELECT status FROM sale_drafts WHERE id=$1`, draft["id"]).Scan(&draftStatus); err != nil || draftStatus != "completed" {
		t.Fatalf("draft completion: %s %v", draftStatus, err)
	}
	delete(sale, "draft_id")
	delete(sale, "draft_version")
	// 5 permintaan paralel berkunci sama → tepat satu transaksi.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, r := cashier.do("POST", "/api/v1/sales/finalize", sale, key("k2")); code != 201 {
				t.Errorf("paralel harus 201, dapat %d %v", code, r)
			}
		}()
	}
	wg.Wait()

	var sales, stock int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM sales`).Scan(&sales)
	_ = db.QueryRow(ctx, `SELECT qty_milli FROM inventory_balances`).Scan(&stock)
	if sales != 2 || stock != 8000 {
		t.Fatalf("harus 2 transaksi (k1,k2) dan stok 8000, dapat %d & %d", sales, stock)
	}
	if code, _ := owner.do("PUT", "/api/v1/memberships/me/pin", map[string]string{"pin": "123456"}, nil); code != 200 {
		t.Fatalf("PIN: %d", code)
	}
	voidPath := "/api/v1/sales/" + r1["sale_id"].(string) + "/void"
	voidBody := map[string]any{"reason": "salah input"}
	if code, _ := cashier.do("POST", voidPath, voidBody, key("void-1")); code != 403 {
		t.Fatalf("void without approval: %d", code)
	}
	voidBody["approval"] = map[string]string{"email": "owner@a.test", "pin": "123456"}
	for i := 0; i < 2; i++ {
		if code, res := cashier.do("POST", voidPath, voidBody, key("void-1")); code != 200 {
			t.Fatalf("void replay: %d %v", code, res)
		}
	}
	if err := db.QueryRow(ctx, `SELECT qty_milli FROM inventory_balances`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 9000 {
		t.Fatalf("void replay restored twice: %d", stock)
	}
}
