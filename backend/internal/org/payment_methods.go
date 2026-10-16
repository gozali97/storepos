package org

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// Konfigurasi hanya metode manual MVP. Kode stabil menjaga histori dan klasifikasi tunai.
type PaymentMethod struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func DefaultPaymentMethods() []PaymentMethod {
	return []PaymentMethod{{"cash", "Tunai", true}, {"qris", "QRIS manual", true}, {"card", "Kartu manual", true}, {"transfer", "Transfer manual", true}}
}
func ValidatePaymentMethods(methods []PaymentMethod) error {
	if len(methods) != 4 {
		return errors.New("empat metode manual wajib dicantumkan")
	}
	seen := map[string]bool{}
	active := false
	for _, m := range methods {
		if m.Code != "cash" && m.Code != "qris" && m.Code != "card" && m.Code != "transfer" {
			return errors.New("kode metode pembayaran tidak dikenal")
		}
		if seen[m.Code] {
			return errors.New("kode metode duplikat")
		}
		seen[m.Code] = true
		if !utf8.ValidString(m.Name) || strings.TrimSpace(m.Name) == "" || utf8.RuneCountInString(m.Name) > 100 {
			return errors.New("nama metode wajib maksimal 100 karakter")
		}
		active = active || m.Enabled
	}
	if !active {
		return errors.New("minimal satu metode harus aktif")
	}
	return nil
}
func LoadPaymentMethods(ctx context.Context, db database.Getter, biz idkit.ID) ([]PaymentMethod, int, error) {
	var raw []byte
	var version int
	if err := db.QueryRow(ctx, `SELECT settings->'payment_methods',payment_methods_version FROM businesses WHERE id=$1`, biz).Scan(&raw, &version); err != nil {
		return nil, 0, err
	}
	out := DefaultPaymentMethods()
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, 0, err
		}
	}
	if err := ValidatePaymentMethods(out); err != nil {
		return nil, 0, err
	}
	return out, version, nil
}
