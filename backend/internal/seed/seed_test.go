package seed

import (
	"context"
	"testing"

	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestDefinitionCoversEveryRoleAndUsesWorkingPassword(t *testing.T) {
	if Password != "password789" {
		t.Fatalf("password seed = %q", Password)
	}
	hash, err := auth.Hash(Password)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := auth.Verify(Password, hash)
	if err != nil || !ok {
		t.Fatalf("password seed tidak lolos verifikasi: ok=%v err=%v", ok, err)
	}

	wantRoles := map[string]bool{"owner": false, "manager": false, "cashier": false, "warehouse": false, "kitchen": false, "pharmacist": false}
	seenEmail := map[string]bool{}
	for _, account := range accounts {
		if seenEmail[account.email] {
			t.Errorf("email ganda: %s", account.email)
		}
		seenEmail[account.email] = true
		if _, ok := wantRoles[account.role]; !ok {
			t.Errorf("role tidak dikenal: %s", account.role)
		} else {
			wantRoles[account.role] = true
		}
	}
	for role, exists := range wantRoles {
		if !exists {
			t.Errorf("role %s belum memiliki akun", role)
		}
	}
	for _, business := range businesses {
		if len(business.categories) == 0 || len(business.products) == 0 {
			t.Errorf("master katalog %s kosong", business.name)
		}
	}
}

func TestApplySeedsEveryMasterAndRoleIdempotently(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		result, err := Apply(ctx, pool)
		if err != nil {
			t.Fatalf("Apply ke-%d: %v", i+1, err)
		}
		if result.Businesses != 3 || len(result.Accounts) != 6 {
			t.Fatalf("hasil seed tidak lengkap: %+v", result)
		}
	}

	db := pool.Underlying()
	wants := map[string]int{
		"businesses": 3, "branches": 3, "registers": 3, "users": 6,
		"memberships": 8, "membership_branches": 8, "categories": 6,
		"products": 9, "variants": 9, "variant_barcodes": 18,
		"purchase_units": 9, "suppliers": 3, "customers": 3,
		"tax_rules": 3, "inventory_balances": 9, "stock_minimums": 9,
		"kitchen_stations": 2, "kitchen_category_routes": 2,
	}
	for table, want := range wants {
		var got int
		if err := db.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("hitung %s: %v", table, err)
		}
		if got != want {
			t.Errorf("%s = %d, ingin %d", table, got, want)
		}
	}

	for _, business := range businesses {
		var active int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM tax_rules WHERE business_id=$1 AND active`, business.id).Scan(&active); err != nil || active != 1 {
			t.Fatalf("pajak tenant %s: %d %v", business.name, active, err)
		}
	}
	rows, err := db.Query(ctx, `SELECT u.email,u.password_hash,m.role
		FROM users u JOIN memberships m ON m.user_id=u.id
		WHERE u.email<>'owner@storepos.test' OR m.business_id='01990000-0000-7000-8000-000000000101'
		ORDER BY u.email`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	roles := map[string]bool{}
	for rows.Next() {
		var email, hash, role string
		if err := rows.Scan(&email, &hash, &role); err != nil {
			t.Fatal(err)
		}
		ok, err := auth.Verify(Password, hash)
		if err != nil || !ok {
			t.Errorf("password %s tidak valid: ok=%v err=%v", email, ok, err)
		}
		roles[role] = true
	}
	for _, role := range []string{"owner", "manager", "cashier", "warehouse", "kitchen", "pharmacist"} {
		if !roles[role] {
			t.Errorf("role %s tidak punya akun", role)
		}
	}
}
