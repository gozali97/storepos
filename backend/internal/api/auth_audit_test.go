package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"strings"
	"testing"
)

func TestAuthenticationAuditAndBusinessRequestMetadata(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@authaudit.test")
	base := newServer(t, pool)
	owner := login(t, base, "owner@authaudit.test", biz.String())
	for _, email := range []string{"owner@authaudit.test", "unknown@authaudit.test"} {
		if code, _ := owner.do("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "never-log-this-secret"}, nil); code != 401 {
			t.Fatalf("wrong login: %d", code)
		}
	}
	if code, _ := owner.do("POST", "/api/v1/customers", map[string]string{"name": "Do not audit this name"}, nil); code != 201 {
		t.Fatalf("business action: %d", code)
	}
	if code, _ := owner.do("POST", "/api/v1/auth/logout", map[string]string{}, nil); code != 200 {
		t.Fatalf("logout: %d", code)
	}
	ctx := context.Background()
	db := pool.Underlying()
	rows, err := db.Query(ctx, `SELECT action,business_id::text,actor_user_id::text,host(ip),user_agent,COALESCE(before_jsonb::text,''),COALESCE(after_jsonb::text,''),COALESCE(reason,'') FROM audit_logs ORDER BY occurred_at,id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var action string
		var business, actor, ip, ua *string
		var before, after, reason string
		if err := rows.Scan(&action, &business, &actor, &ip, &ua, &before, &after, &reason); err != nil {
			t.Fatal(err)
		}
		counts[action]++
		if ip == nil || ua == nil {
			t.Fatalf("missing metadata: %s", action)
		}
		if strings.HasPrefix(action, "auth.") && business != nil {
			t.Fatalf("global auth exposed as tenant: %s", action)
		}
		if action == "auth.login" && actor == nil {
			t.Fatal("login actor missing")
		}
		text := before + after + reason
		if strings.Contains(text, "never-log") || strings.Contains(text, "@authaudit") || strings.Contains(text, "Do not audit") {
			t.Fatal("PII/secret audit leak")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if counts["auth.login"] != 1 || counts["auth.login_failed"] != 2 || counts["auth.logout"] != 1 || counts["customer.created"] != 1 {
		t.Fatal(counts)
	}
}
