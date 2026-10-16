// Package idempotency menjamin satu Idempotency-Key hanya menghasilkan satu efek (CORE-017, PRD §13.12).
//
// Begin dan Complete WAJIB dipanggil di transaksi yang sama dengan perubahan datanya:
// bila transaksi rollback, kunci ikut hilang sehingga klien boleh mencoba lagi; bila commit,
// kunci dan hasil tersimpan bersama. Permintaan paralel berkunci sama menunggu pada indeks
// unik lalu melihat hasil yang sudah selesai.
package idempotency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// ErrConflict: kunci sama dipakai untuk endpoint atau isi permintaan yang berbeda.
var ErrConflict = errors.New("Idempotency-Key sudah dipakai untuk permintaan yang berbeda")

// Stored adalah respons tersimpan yang diputar ulang.
type Stored struct {
	Status int
	Body   []byte
}

// Begin mendaftarkan kunci. Mengembalikan (nil, nil) bila ini permintaan pertama,
// atau respons tersimpan bila kunci sudah selesai dengan isi yang sama.
func Begin(ctx context.Context, db database.Getter, biz idkit.ID, key, endpoint string, hash []byte) (*Stored, error) {
	tag, err := db.Exec(ctx,
		`INSERT INTO idempotency_keys (business_id,key,endpoint,request_hash,expires_at)
		 VALUES ($1,$2,$3,$4, now() + interval '72 hours') ON CONFLICT DO NOTHING`,
		biz, key, endpoint, hash)
	if err != nil {
		return nil, fmt.Errorf("idempotency: daftar kunci: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}
	var (
		ep, st  string
		h, body []byte
		code    *int
	)
	if err := db.QueryRow(ctx,
		`SELECT endpoint, status, request_hash, response_status, response_body
		 FROM idempotency_keys WHERE business_id=$1 AND key=$2`, biz, key).Scan(&ep, &st, &h, &code, &body); err != nil {
		return nil, fmt.Errorf("idempotency: baca kunci: %w", err)
	}
	if ep != endpoint || !bytes.Equal(h, hash) || st != "completed" || code == nil {
		return nil, ErrConflict
	}
	return &Stored{Status: *code, Body: body}, nil
}

// Complete menyimpan respons agar dapat diputar ulang.
func Complete(ctx context.Context, db database.Getter, biz idkit.ID, key string, status int, body []byte) error {
	if _, err := db.Exec(ctx,
		`UPDATE idempotency_keys SET status='completed', response_status=$3, response_body=$4::jsonb, completed_at=now()
		 WHERE business_id=$1 AND key=$2`, biz, key, status, json.RawMessage(body)); err != nil {
		return fmt.Errorf("idempotency: simpan hasil: %w", err)
	}
	return nil
}
