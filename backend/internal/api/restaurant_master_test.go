package apiserver

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestRestaurantAreaTableLifecycleAndOpenOrderProtection(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Restoran", "owner@master.restaurant.test")
	branch := idkit.New()
	db := pool.Underlying()
	if _, err := db.Exec(t.Context(), `UPDATE businesses SET business_type='restaurant',enabled_modules=ARRAY['restaurant'] WHERE id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'R','Restoran')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	c := login(t, base, "owner@master.restaurant.test", biz.String())

	code, area := c.do("POST", "/api/v1/restaurant/areas", map[string]any{"branch_id": branch.String(), "name": "Indoor"}, nil)
	if code != 201 {
		t.Fatal(code, area)
	}
	aid := area["area_id"].(string)
	code, table := c.do("POST", "/api/v1/restaurant/tables", map[string]any{"branch_id": branch.String(), "area_id": aid, "name": "Meja 1", "capacity": 4}, nil)
	if code != 201 {
		t.Fatal(code, table)
	}
	tid := table["table_id"].(string)

	if code, _ := c.do("PUT", "/api/v1/restaurant/areas/"+aid, map[string]any{"name": "Dalam"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatalf("edit area: %d", code)
	}
	if code, _ := c.do("PUT", "/api/v1/restaurant/tables/"+tid, map[string]any{"name": "Meja A", "capacity": 6}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatalf("edit meja: %d", code)
	}
	// Area tidak boleh diarsipkan sebelum meja aktifnya diarsipkan.
	if code, _ := c.do("DELETE", "/api/v1/restaurant/areas/"+aid, nil, map[string]string{"If-Match": "2"}); code != 409 {
		t.Fatalf("area dengan meja aktif: %d", code)
	}
	if code, _ := c.do("DELETE", "/api/v1/restaurant/tables/"+tid, nil, map[string]string{"If-Match": "2"}); code != 200 {
		t.Fatalf("arsip meja: %d", code)
	}
	if code, _ := c.do("DELETE", "/api/v1/restaurant/areas/"+aid, nil, map[string]string{"If-Match": "2"}); code != 200 {
		t.Fatalf("arsip area: %d", code)
	}
	if code, _ := c.do("POST", "/api/v1/restaurant/areas/"+aid+"/restore", map[string]any{}, map[string]string{"If-Match": "3"}); code != 200 {
		t.Fatalf("pulihkan area: %d", code)
	}
	if code, _ := c.do("POST", "/api/v1/restaurant/tables/"+tid+"/restore", map[string]any{}, map[string]string{"If-Match": "3"}); code != 200 {
		t.Fatalf("pulihkan meja: %d", code)
	}

	// Order terbuka melindungi meja dari arsip.
	code, order := c.do("POST", "/api/v1/restaurant/orders", map[string]any{"branch_id": branch.String(), "table_id": tid, "order_type": "DINE_IN"}, nil)
	if code != 201 {
		t.Fatal(code, order)
	}
	if code, _ := c.do("DELETE", "/api/v1/restaurant/tables/"+tid, nil, map[string]string{"If-Match": "4"}); code != 409 {
		t.Fatalf("meja dengan order terbuka: %d", code)
	}

	code, listed := c.do("GET", "/api/v1/restaurant/tables?branch_id="+branch.String()+"&include_archived=true", nil, nil)
	if code != 200 || len(listed["tables"].([]any)) != 1 {
		t.Fatalf("daftar meja: %d %v", code, listed)
	}
}
