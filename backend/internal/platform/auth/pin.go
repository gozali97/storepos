package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
)

// PIN mengikuti CORE-006: 6 digit, disimpan sebagai hash, dikunci 15 menit
// setelah 5 kali gagal berturut-turut.
const (
	PINLength       = 6
	PINMaxFailures  = 5
	PINLockDuration = 15 * 60 // detik
)

var (
	ErrPINFormat   = errors.New("PIN harus 6 digit angka")
	ErrPINLocked   = errors.New("PIN terkunci sementara")
	ErrPINMismatch = errors.New("PIN salah")
)

// HashPIN menghasilkan hash PIN memakai argon2id dengan parameter yang sama
// seperti kata sandi (PIN entropinya rendah, jadi lapisan pertahanan adalah
// rate limit + lockout, bukan hanya kekuatan hash).
func HashPIN(pin string) (string, error) {
	if !validPIN(pin) {
		return "", ErrPINFormat
	}
	return Hash(pin)
}

// VerifyPIN memeriksa PIN terhadap hash.
func VerifyPIN(pin, hash string) (bool, error) {
	if !validPIN(pin) {
		return false, ErrPINFormat
	}
	return Verify(pin, hash)
}

func validPIN(pin string) bool {
	if len(pin) != PINLength {
		return false
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// GeneratePIN membuat PIN acak 6 digit memakai crypto/rand.
// Dipakai untuk alur "lupa PIN" yang mengharuskan pengguna mengganti PIN.
func GeneratePIN() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// ConstantTimeEqualPIN membandingkan dua PIN dalam waktu konstan.
// Berguna untuk alur perbandingan langsung tanpa hash (tidak disarankan).
func ConstantTimeEqualPIN(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
