package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestInvitationAcceptanceAndRestrictions(t *testing.T) {
	pool := testdb.New(t)
	a := seedBusiness(t, pool, "A", "owner@invite.test")
	b := seedBusiness(t, pool, "B", "recipient@invite.test")
	seedUser(t, pool, "manager@invite.test", "manager", a)
	base := newServer(t, pool)
	owner := login(t, base, "owner@invite.test", a.String())
	recipient := login(t, base, "recipient@invite.test", b.String())
	manager := login(t, base, "manager@invite.test", a.String())
	body := map[string]any{"email": "recipient@invite.test", "role": "cashier", "branch_ids": []string{}}
	if code, _ := manager.do("POST", "/api/v1/invitations", map[string]string{"email": "recipient@invite.test", "role": "owner"}, nil); code != 403 {
		t.Fatalf("manager escalation: %d", code)
	}
	code, invite := owner.do("POST", "/api/v1/invitations", body, nil)
	if code != 201 {
		t.Fatal(code, invite)
	}
	token := invite["token"].(string)
	if code, _ := recipient.do("POST", "/api/v1/invitations/accept", map[string]string{"token": token}, nil); code != 403 {
		t.Fatalf("unverified email: %d", code)
	}
	if _, err := pool.Underlying().Exec(context.Background(), `UPDATE users SET email_verified_at=now() WHERE email='recipient@invite.test'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := owner.do("POST", "/api/v1/invitations/accept", map[string]string{"token": token}, nil); code != 403 {
		t.Fatalf("recipient mismatch: %d", code)
	}
	if code, r := recipient.do("POST", "/api/v1/invitations/accept", map[string]string{"token": token}, nil); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := recipient.do("POST", "/api/v1/invitations/accept", map[string]string{"token": token}, nil); code != 409 {
		t.Fatalf("reused token: %d", code)
	}
	recipient.biz = a.String()
	if code, _ := recipient.do("GET", "/api/v1/catalog/search", nil, nil); code != 200 {
		t.Fatalf("joined tenant: %d", code)
	}
	body["email"] = "other@invite.test"
	code, revoked := owner.do("POST", "/api/v1/invitations", body, nil)
	if code != 201 {
		t.Fatal(code, revoked)
	}
	if code, _ := owner.do("DELETE", "/api/v1/invitations/"+revoked["invitation_id"].(string), nil, nil); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if code, _ := recipient.do("POST", "/api/v1/invitations/accept", map[string]string{"token": revoked["token"].(string)}, nil); code != 409 {
		t.Fatalf("revoked: %d", code)
	}
	code, expired := owner.do("POST", "/api/v1/invitations", body, nil)
	if code != 201 {
		t.Fatal(code, expired)
	}
	if _, err := pool.Underlying().Exec(context.Background(), `UPDATE invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, expired["invitation_id"]); err != nil {
		t.Fatal(err)
	}
	if code, _ := recipient.do("POST", "/api/v1/invitations/accept", map[string]string{"token": expired["token"].(string)}, nil); code != 409 {
		t.Fatalf("expired: %d", code)
	}
	var leaks int
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE after_jsonb::text LIKE $1 OR before_jsonb::text LIKE $1`, "%"+token+"%").Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("token in audit: %d %v", leaks, err)
	}
}
