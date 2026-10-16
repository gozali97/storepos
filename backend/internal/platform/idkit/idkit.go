// Package idkit menghasilkan UUIDv7 (RFC 9562 §5.7) untuk ID entitas.
// UUIDv7 memiliki prefix timestamp sehingga ramah indeks B-tree PostgreSQL (§13.13).
package idkit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ID adalah UUID 16 byte.
type ID [16]byte

// NewV7 menghasilkan UUIDv7 baru.
func NewV7() ID { return newV7At(time.Now()) }

func newV7At(now time.Time) ID {
	var b ID
	ms := now.UTC().UnixMilli()
	// 48-bit timestamp big-endian
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := rand.Read(b[6:]); err != nil {
		panic("rand.Read failed: " + err.Error())
	}
	// version 7 di nibble atas byte 6
	b[6] = (b[6] & 0x0f) | 0x70
	// variant 10 di byte 8
	b[8] = (b[8] & 0x3f) | 0x80
	return b
}

// String memformat ID sebagai 8-4-4-4-12 heks.
func (id ID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], id[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], id[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], id[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], id[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], id[10:16])
	return string(buf[:])
}

// Parse membaca string UUID canonical.
func Parse(s string) (ID, error) {
	var id ID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return id, fmt.Errorf("uuid: panjang/format salah: %q", s)
	}
	parts := []string{s[0:8], s[9:13], s[14:18], s[19:23], s[24:36]}
	offsets := []int{0, 4, 6, 8, 10}
	for i, p := range parts {
		if _, err := hex.Decode(id[offsets[i]:], []byte(p)); err != nil {
			return id, errors.New("uuid: karakter heks tidak valid")
		}
	}
	return id, nil
}

// MarshalText memenuhi encoding.TextMarshaler.
func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

// UnmarshalText memenuhi encoding.TextUnmarshaler.
func (id *ID) UnmarshalText(b []byte) error {
	v, err := Parse(string(b))
	if err != nil {
		return err
	}
	*id = v
	return nil
}

// IsZero mengembalikan true bila semua byte nol.
func (id ID) IsZero() bool { return id == ID{} }

// Version mengembalikan nibble versi (seharusnya 7 untuk ID yang kita hasilkan).
func (id ID) Version() int { return int(id[6] >> 4) }

// New adalah alias NewV7 untuk pemakaian umum di repository.
func New() ID { return NewV7() }
