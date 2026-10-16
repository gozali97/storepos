package apiserver

import (
	"context"
	"errors"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/platform/testdb"
	"sync"
	"testing"
	"time"
)

func TestParallelPINFailuresStopAtLockout(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@pin.test")
	seedUser(t, pool, "cashier@pin.test", "cashier", biz)
	ctx := context.Background()
	var mid, actor idkit.ID
	if err := pool.Underlying().QueryRow(ctx, `SELECT m.id FROM memberships m JOIN users u ON u.id=m.user_id WHERE u.email='owner@pin.test'`).Scan(&mid); err != nil {
		t.Fatal(err)
	}
	if err := pool.Underlying().QueryRow(ctx, `SELECT id FROM users WHERE email='cashier@pin.test'`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err := org.NewRepo(pool.Underlying()).SetPIN(ctx, mid, "123456"); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(pool, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	tc := tenant.Context{BusinessID: biz, UserID: actor}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.approver(ctx, tc, &approvalReq{Email: "owner@pin.test", PIN: "000000"})
			if !errors.Is(err, org.ErrApprovalDenied) && !errors.Is(err, org.ErrPINLocked) {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	var failures int
	if err := pool.Underlying().QueryRow(ctx, `SELECT pin_fail_count FROM memberships WHERE id=$1`, mid).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 5 {
		t.Fatalf("lockout must stop at five: %d", failures)
	}
	if _, err := s.approver(ctx, tc, &approvalReq{Email: "owner@pin.test", PIN: "123456"}); !errors.Is(err, org.ErrPINLocked) {
		t.Fatalf("locked: %v", err)
	}
	if _, err := pool.Underlying().Exec(ctx, `UPDATE memberships SET pin_locked_until=now()-interval '1 second' WHERE id=$1`, mid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.approver(ctx, tc, &approvalReq{Email: "owner@pin.test", PIN: "000000"}); !errors.Is(err, org.ErrApprovalDenied) {
		t.Fatal(err)
	}
	if err := pool.Underlying().QueryRow(ctx, `SELECT pin_fail_count FROM memberships WHERE id=$1`, mid).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 1 {
		t.Fatalf("expired lock starts new attempts: %d", failures)
	}
}
