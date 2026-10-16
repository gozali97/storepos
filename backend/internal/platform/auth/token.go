package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// TokenPrefix mempermudah identifikasi jenis token di log dan saat debugging.
const (
	PrefixSession    = "sps_"
	PrefixCSRF       = "spc_"
	PrefixInvitation = "spi_"
	PrefixReceipt    = "spr_"
)

// newToken menghasilkan string acak 32 byte yang di-encode base64url tanpa padding.
func newToken(prefix string) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// NewSessionToken menghasilkan token sesi yang dikirim ke browser.
// Token mentah TIDAK pernah disimpan di database (PRD §16.5).
func NewSessionToken() (string, error) { return newToken(PrefixSession) }

// NewCSRFToken menghasilkan token CSRF untuk pola double-submit cookie.
func NewCSRFToken() (string, error) { return newToken(PrefixCSRF) }

// NewInvitationToken menghasilkan token undangan yang dikirim via email/WhatsApp.
func NewInvitationToken() (string, error) { return newToken(PrefixInvitation) }

func NewReceiptToken() (string, error) { return newToken(PrefixReceipt) }

// HashToken menghitung SHA-256 dari token untuk disimpan di database.
//
// Token sesi bersifat acak penuh (256 bit), sehingga SHA-256 tanpa salt sudah
// memadai — berbeda dengan kata sandi yang harus memakai argon2id. Ini menjaga
// lookup sesi tetap satu indeks B-tree tanpa kompromi keamanan.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
