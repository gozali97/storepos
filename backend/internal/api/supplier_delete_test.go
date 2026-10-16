package apiserver

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

// CORE-043: master yang dirujuk dokumen hanya boleh diarsipkan; DELETE harus 409, bukan 500 dari FK database.
func TestSupplierReferencedByPharmacyBatchReceiptCannotBeDeleted(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Apotek", "owner@sup.delete.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@sup.delete.test", biz.String())
	code, sup := c.do("POST", "/api/v1/suppliers", map[string]any{"name": "Pemasok Obat"}, nil)
	if code != 201 {
		t.Fatal(code, sup)
	}
	supplier := sup["id"].(string)

	// Supplier yang belum dipakai boleh dihapus permanen.
	code, free := c.do("POST", "/api/v1/suppliers", map[string]any{"name": "Belum Dipakai"}, nil)
	if code != 201 {
		t.Fatal(code, free)
	}
	if code, _ := c.do("DELETE", "/api/v1/suppliers/"+free["id"].(string), nil, map[string]string{"If-Match": "1"}); code != 204 && code != 200 {
		t.Fatalf("supplier tak terpakai harus bisa dihapus: %d", code)
	}

	// Pakai supplier pada penerimaan batch apotek.
	db := pool.Underlying()
	ctx := t.Context()
	user, product, variant, batch, branch := idkit.New(), idkit.New(), idkit.New(), idkit.New(), idkit.New()
	var owner idkit.ID
	if err := db.QueryRow(ctx, `SELECT created_by FROM businesses WHERE id=$1`, biz).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	_ = user
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'P','Pusat')`, []any{branch, biz}},
		{`INSERT INTO products(id,business_id,name,track_stock) VALUES($1,$2,'Obat',true)`, []any{product, biz}},
		{`INSERT INTO variants(id,business_id,product_id,name,sku,base_unit,price_cents) VALUES($1,$2,$3,'','OB-1','strip',100)`, []any{variant, biz, product}},
		{`INSERT INTO pharmacy_batches(id,business_id,variant_id,batch_number,expiry_date) VALUES($1,$2,$3,'B1',current_date+100)`, []any{batch, biz, variant}},
		{`INSERT INTO pharmacy_batch_receipts(id,business_id,branch_id,batch_id,supplier_id,invoice_number,qty_milli,unit_cost_cents,status,created_by) VALUES(gen_random_uuid(),$1,$2,$3,$4::uuid,'INV',1000,100,'SELLABLE',$5)`, []any{biz, branch, batch, supplier, owner}},
	} {
		if _, err := db.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	code, out := c.do("DELETE", "/api/v1/suppliers/"+supplier, nil, map[string]string{"If-Match": "1"})
	if code != 409 {
		t.Fatalf("supplier yang dipakai penerimaan batch harus 409 (arsipkan), dapat %d: %v", code, out)
	}
}

// CORE-043 AC: produk yang pernah dipakai hanya bisa diarsipkan; tidak ada jalur hapus permanen untuk produk/varian.
func TestProductsAndVariantsHaveNoPermanentDeleteRoute(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@nodelete.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@nodelete.test", biz.String())
	code, out := c.do("POST", "/api/v1/products", map[string]any{"name": "Produk", "variants": []map[string]any{{"sku": "ND-1", "price_cents": "100"}}}, nil)
	if code != 201 {
		t.Fatal(code, out)
	}
	pid := out["id"].(string)
	vid := out["variants"].([]any)[0].(map[string]any)["id"].(string)
	for _, path := range []string{"/api/v1/products/" + pid, "/api/v1/variants/" + vid} {
		if code, _ := c.do("DELETE", path, nil, map[string]string{"If-Match": "1"}); code != 404 && code != 405 {
			t.Errorf("DELETE %s harus ditolak (404/405), dapat %d", path, code)
		}
		if code, _ := c.do("GET", path, nil, nil); code != 200 {
			t.Errorf("%s harus tetap ada: %d", path, code)
		}
	}
}
