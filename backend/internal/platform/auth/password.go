// Package password menyediakan hashing kata sandi dengan argon2id.
//
// Format tersimpan (PHC string):
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>
//
// Parameter mengikuti rekomendasi OWASP 2024 untuk argon2id: memori 64 MiB,
// 3 iterasi, paralelisme 2. Kata sandi tidak boleh keluar dari proses ini.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memoryKiB  = 64 * 1024
	iterations = 3
	parallel   = 2
	saltLen    = 16
	keyLen     = 32

	// minLength mengikuti NFR-011.
	minLength = 8
)

var (
	ErrTooShort    = errors.New("kata sandi minimal 8 karakter")
	ErrEmpty       = errors.New("kata sandi tidak boleh kosong")
	ErrInvalidHash = errors.New("format hash tidak dikenal")
	ErrCommon      = errors.New("kata sandi terlalu umum; pilih yang lebih sulit ditebak")
)

// commonPasswords adalah daftar kecil kata sandi yang paling sering bocor (NFR-011).
// shortcut: daftar pendek tertanam; ganti dengan daftar 10 ribu teratas atau layanan k-anonymity bila perlu.
var commonPasswords = map[string]bool{
	"12345678": true, "123456789": true, "1234567890": true, "11111111": true, "00000000": true,
	"password": true, "password1": true, "password123": true, "passw0rd": true, "p@ssw0rd": true,
	"qwertyui": true, "qwerty123": true, "qwertyuiop": true, "iloveyou": true, "admin123": true,
	"abc12345": true, "abcd1234": true, "welcome1": true, "letmein1": true, "monkey123": true,
	"katasandi": true, "sandi123": true, "rahasia123": true, "indonesia": true, "indonesia1": true,
	"storepos": true, "storepos1": true, "storepos123": true, "87654321": true, "12341234": true,
}

// VerifyPolicy memeriksa aturan minimal kata sandi (NFR-011).
// Kata sandi umum (daftar tertanam, tidak peka huruf besar/kecil) juga ditolak.
func VerifyPolicy(plain string) error {
	if plain == "" {
		return ErrEmpty
	}
	// Hitung rune, bukan byte, agar pengguna tidak dirugikan oleh akses non-ASCII.
	if len([]rune(plain)) < minLength {
		return ErrTooShort
	}
	if commonPasswords[strings.ToLower(plain)] {
		return ErrCommon
	}
	return nil
}

// Hash menghasilkan PHC string dari kata sandi.
func Hash(plain string) (string, error) {
	if plain == "" {
		return "", ErrEmpty
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, iterations, memoryKiB, parallel, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify membandingkan kata sandi dengan PHC string secara konstan-waktu.
func Verify(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrInvalidHash
	}
	var m uint32
	var t, p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(plain), salt, uint32(t), m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash melaporkan apakah hash tersimpan memakai parameter lama.
func NeedsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return true
	}
	var m uint32
	var t, p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return true
	}
	return m != memoryKiB || int(t) != iterations || int(p) != parallel
}
