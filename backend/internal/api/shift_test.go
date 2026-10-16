package apiserver

import (
	"context"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestSupervisorApprovalAndPINLockout(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko A", "owner@a.test")
	seedUser(t, pool, "kasir@a.test", "cashier", biz)
	base := newServer(t, pool)
	ctx := context.Background()
	db := pool.Underlying()

	var branch, reg string
	_ = db.QueryRow(ctx, `INSERT INTO branches (id,business_id,code,name) VALUES (gen_random_uuid(),$1,'JKT','Pusat') RETURNING id::text`, biz).Scan(&branch)
	_ = db.QueryRow(ctx, `INSERT INTO registers (id,business_id,branch_id,name) VALUES (gen_random_uuid(),$1,$2,'K1') RETURNING id::text`, biz, branch).Scan(&reg)
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches (membership_id, business_id, branch_id)
		SELECT m.id, m.business_id, $2::uuid FROM memberships m JOIN users u ON u.id=m.user_id
		WHERE u.email='kasir@a.test' AND m.business_id=$1`, biz, branch); err != nil {
		t.Fatal(err)
	}
	owner := login(t, base, "owner@a.test", biz.String())
	cashier := login(t, base, "kasir@a.test", biz.String())

	// Kasir tidak boleh punya PIN atasan; Pemilik boleh. PIN harus 6 digit.
	if code, _ := cashier.do("PUT", "/api/v1/memberships/me/pin", map[string]string{"pin": "123456"}, nil); code != 403 {
		t.Fatalf("kasir set PIN harus 403, dapat %d", code)
	}
	if code, _ := owner.do("PUT", "/api/v1/memberships/me/pin", map[string]string{"pin": "12ab"}, nil); code != 422 {
		t.Fatalf("PIN tidak valid harus 422, dapat %d", code)
	}
	if code, _ := owner.do("PUT", "/api/v1/memberships/me/pin", map[string]string{"pin": "123456"}, nil); code != 200 {
		t.Fatalf("owner set PIN harus 200, dapat %d", code)
	}
	if code, _ := cashier.do("POST", "/api/v1/shifts/open", map[string]any{"branch_id": branch, "register_id": reg, "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatalf("buka shift: %d", code)
	}

	out := func(approval map[string]string) (int, map[string]any) {
		body := map[string]any{"branch_id": branch, "register_id": reg, "kind": "out", "amount_cents": "60000000", "reason": "setor ke bank"}
		if approval != nil {
			body["approval"] = approval
		}
		return cashier.do("POST", "/api/v1/shifts/cash", body, nil)
	}
	errCode := func(r map[string]any) string { return r["error"].(map[string]any)["code"].(string) }

	// Di atas ambang tanpa persetujuan → 403 APPROVAL_REQUIRED.
	if code, r := out(nil); code != 403 || errCode(r) != "APPROVAL_REQUIRED" {
		t.Fatalf("tanpa O harus APPROVAL_REQUIRED, dapat %d %v", code, r)
	}
	// PIN benar → berhasil.
	if code, r := out(map[string]string{"email": "owner@a.test", "pin": "123456"}); code != 201 {
		t.Fatalf("dengan PIN benar harus 201, dapat %d %v", code, r)
	}
	// Atasan tidak boleh menyetujui dirinya sendiri sebagai pelaku.
	if code, _ := owner.do("POST", "/api/v1/shifts/cash", map[string]any{"branch_id": branch, "register_id": reg,
		"kind": "out", "amount_cents": "60000000", "reason": "x",
		"approval": map[string]string{"email": "owner@a.test", "pin": "123456"}}, nil); code != 403 {
		t.Fatalf("persetujuan diri sendiri harus 403, dapat %d", code)
	}
	// 5 kali PIN salah → terkunci; PIN benar pun ditolak 429.
	for i := 0; i < 5; i++ {
		if code, _ := out(map[string]string{"email": "owner@a.test", "pin": "000000"}); code != 403 {
			t.Fatalf("PIN salah ke-%d harus 403, dapat %d", i+1, code)
		}
	}
	if code, _ := out(map[string]string{"email": "owner@a.test", "pin": "123456"}); code != 429 {
		t.Fatalf("setelah 5 gagal harus 429 walau PIN benar, dapat %d", code)
	}
	// Kasir tidak dapat menutup paksa shift lain (force) → 403.
	if code, _ := cashier.do("POST", "/api/v1/shifts/close", map[string]any{"branch_id": branch, "register_id": reg,
		"counted_cash_cents": "0", "force": true}, nil); code != 403 {
		t.Fatalf("force oleh kasir harus 403, dapat %d", code)
	}
}
