package apiserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/leads"
	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (s *Server) createLead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		httpx.WriteError(w, r, errs.Validation("Gunakan body JSON."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	var req leads.Request
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	started, err := time.Parse(time.RFC3339Nano, req.StartedAt)
	if err != nil || time.Since(started) < 3*time.Second {
		httpx.WriteError(w, r, errs.Validation("Periksa formulir sebelum mengirim kembali."))
		return
	}
	if len(req.Website) > 0 {
		httpx.JSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
		return
	}
	if err := req.Validate(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key, err := idkit.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key harus UUID."))
		return
	}
	// Waktu buka form tidak mengubah identitas isi pada retry setelah muat ulang.
	req.StartedAt, req.Website = "", ""
	payload, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(payload)
	peer := auth.HashToken("lead-peer:" + ClientIP(r))
	limited := false
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "lead-key:"+key.String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "lead:"+req.Phone+":"+req.BusinessType); err != nil {
			return err
		}
		var existingHash []byte
		err := tx.QueryRow(ctx, `SELECT request_hash FROM platform.lead_requests WHERE idempotency_key=$1`, key).Scan(&existingHash)
		if err == nil {
			if !bytes.Equal(existingHash, hash[:]) {
				return errs.New(errs.CodeIdempotencyConflict, "Kunci pengiriman sudah dipakai untuk isian berbeda.", http.StatusConflict)
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `INSERT INTO platform.lead_limits(peer_hash,window_start,requests) VALUES($1,now(),1)
		ON CONFLICT(peer_hash) DO UPDATE SET
		requests=CASE WHEN platform.lead_limits.window_start<=now()-interval '1 hour' THEN 1 ELSE LEAST(platform.lead_limits.requests+1,6) END,
		window_start=CASE WHEN platform.lead_limits.window_start<=now()-interval '1 hour' THEN now() ELSE platform.lead_limits.window_start END
		RETURNING requests<=5`, peer).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			limited = true
			return nil
		}
		var duplicateID idkit.ID
		err = tx.QueryRow(ctx, `SELECT id FROM platform.leads WHERE phone=$1 AND business_type=$2 AND created_at>now()-interval '10 minutes' ORDER BY created_at DESC LIMIT 1`, req.Phone, req.BusinessType).Scan(&duplicateID)
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO platform.lead_requests(idempotency_key,lead_id,request_hash) VALUES($1,$2,$3)`, key, duplicateID, hash[:])
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		id := idkit.New()
		if _, err := tx.Exec(ctx, `INSERT INTO platform.leads(id,name,phone,business_name,business_type,city,email,branches,terminals,current_system,notes,contact_time,consent,marketing_consent,consent_version,privacy_version,source_page,source_tab,consent_text)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,true,$13,$14,$15,$16,$17,$18)`, id, req.Name, req.Phone, req.BusinessName, req.BusinessType, req.City, req.Email, req.Branches, req.Terminals, req.CurrentSystem, req.Notes, req.ContactTime, req.MarketingConsent, req.ConsentVersion, req.PrivacyVersion, req.SourcePage, req.SourceTab, leads.ConsentText); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO platform.lead_requests(idempotency_key,lead_id,request_hash) VALUES($1,$2,$3)`, key, id, hash[:]); err != nil {
			return err
		}
		// Notifikasi membawa ID saja; worker penyedia TBK-10 mengambil data sesuai izin.
		_, err = tx.Exec(ctx, `INSERT INTO outbox_jobs(id,kind,payload) VALUES($1,'lead.notification',jsonb_build_object('lead_id',$2::text))`, idkit.New(), id.String())
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if limited {
		w.Header().Set("Retry-After", "3600")
		httpx.WriteError(w, r, errs.New(errs.CodeRateLimited, "Batas permintaan demo tercapai. Coba lagi setelah satu jam.", http.StatusTooManyRequests))
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}
