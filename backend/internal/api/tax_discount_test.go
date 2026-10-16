package apiserver

import (
	"github.com/storepos/storepos/internal/platform/testdb"
	"github.com/storepos/storepos/internal/pricing"
	"testing"
)

func TestDiscountPayload(t *testing.T) {
	for _, d := range []discountReq{{"percent", "10001"}, {"amount", "-1"}, {"unknown", "1"}, {"none", "5"}, {"percent", "1.5"}} {
		if _, err := d.parse(); err == nil {
			t.Fatalf("invalid discount accepted: %+v", d)
		}
	}
	d, err := (discountReq{"percent", "1000"}).parse()
	if err != nil || d.Kind != pricing.DiscountPercent || d.Value != 1000 {
		t.Fatalf("discount: %+v %v", d, err)
	}
}

func TestTaxSettingsOwnerOnly(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@tax.test")
	seedUser(t, pool, "cashier@tax.test", "cashier", biz)
	base := newServer(t, pool)
	owner := login(t, base, "owner@tax.test", biz.String())
	cashier := login(t, base, "cashier@tax.test", biz.String())
	rule := map[string]any{"name": "Pajak demo", "rate_bps": 1000, "mode": "exclusive"}
	if code, _ := cashier.do("PUT", "/api/v1/settings/tax", rule, nil); code != 403 {
		t.Fatalf("cashier: %d", code)
	}
	if code, res := owner.do("PUT", "/api/v1/settings/tax", rule, nil); code != 200 {
		t.Fatalf("owner: %d %v", code, res)
	}
	rule["rate_bps"] = -1
	if code, _ := owner.do("PUT", "/api/v1/settings/tax", rule, nil); code != 422 {
		t.Fatalf("invalid rate: %d", code)
	}
}
