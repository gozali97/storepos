package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/leads"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestLandingLeadSubmission(t *testing.T) {
	pool := testdb.New(t)
	s := &Server{pool: pool}
	req := leads.Request{Name: "Pemilik demo", Phone: "0812-3456-7890", BusinessName: "Toko demo", BusinessType: "retail", City: "Jakarta", Consent: true, ConsentVersion: leads.ConsentVersion, PrivacyVersion: leads.PrivacyVersion, StartedAt: time.Now().Add(-time.Minute).Format(time.RFC3339)}
	submit := func(data leads.Request, key string) int {
		body, _ := json.Marshal(data)
		r := httptest.NewRequest("POST", "/api/v1/leads", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		r.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		s.createLead(w, r)
		return w.Code
	}
	key := idkit.New().String()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if status := submit(req, key); status != 202 {
				t.Errorf("retry status=%d", status)
			}
		}()
	}
	wg.Wait()
	var count int
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM platform.leads`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("lead count=%d err=%v", count, err)
	}
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM outbox_jobs WHERE kind='lead.notification'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("outbox count=%d err=%v", count, err)
	}
	duplicateKey := idkit.New().String()
	if status := submit(req, duplicateKey); status != 202 {
		t.Fatalf("dedup status=%d", status)
	}
	if _, err := pool.Underlying().Exec(context.Background(), `UPDATE platform.leads SET created_at=now()-interval '11 minutes'`); err != nil {
		t.Fatal(err)
	}
	if status := submit(req, duplicateKey); status != 202 {
		t.Fatalf("dedup key retry status=%d", status)
	}
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM platform.leads`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dedup key tidak tersimpan: count=%d err=%v", count, err)
	}
	changed := req
	changed.Name = "Berbeda"
	if status := submit(changed, key); status != 409 {
		t.Fatalf("key conflict status=%d", status)
	}
	changed.Consent = false
	if status := submit(changed, idkit.New().String()); status != 422 {
		t.Fatalf("consent status=%d", status)
	}
	for i := 0; i < 4; i++ {
		changed = req
		changed.Phone = fmt.Sprintf("081234567%03d", i)
		want := 202
		if i == 3 {
			want = 429
		}
		if status := submit(changed, idkit.New().String()); status != want {
			t.Fatalf("submit %d status=%d want=%d", i, status, want)
		}
	}
}
