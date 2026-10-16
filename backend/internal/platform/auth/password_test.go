package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("rahasia-super-aman")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("format hash tak terduga: %s", h)
	}
	ok, err := Verify("rahasia-super-aman", h)
	if err != nil || !ok {
		t.Fatalf("verify gagal: ok=%v err=%v", ok, err)
	}
	ok, err = Verify("salah", h)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if ok {
		t.Fatal("kata sandi salah harus gagal")
	}
}

func TestHashIsSaltedUnique(t *testing.T) {
	a, _ := Hash("sama-persis-123")
	b, _ := Hash("sama-persis-123")
	if a == b {
		t.Fatal("hash harus berbeda karena salt acak")
	}
}

func TestVerifyInvalidHash(t *testing.T) {
	if _, err := Verify("x", "bukan-phc-string"); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyPolicy(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", true},
		{"1234567", true},
		{"12345678", true},    // terlalu umum
		{"Password123", true}, // umum, tidak peka huruf besar/kecil
		{"StorePOS", true},
		{"cangkir-kopi-9", false},
		{"kata-sandi-panjang", false},
		{"pässwörd", false}, // 8 rune
		{"pässwö", true},    // 6 rune
	}
	for _, c := range cases {
		err := VerifyPolicy(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("VerifyPolicy(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	h, _ := Hash("abcdefgh")
	if NeedsRehash(h) {
		t.Fatal("hash baru tidak perlu rehash")
	}
	old := "$argon2id$v=19$m=1024,t=1,p=1$c2FsdA$aGFzaA"
	if !NeedsRehash(old) {
		t.Fatal("hash lama harus perlu rehash")
	}
}
