package auth

import (
	"strings"
	"testing"
)

func TestHashPINRejectsBadFormat(t *testing.T) {
	bad := []string{"", "12345", "1234567", "abcdef", "12 456", "12345x"}
	for _, p := range bad {
		if _, err := HashPIN(p); err == nil {
			t.Errorf("HashPIN(%q) seharusnya error", p)
		}
	}
}

func TestHashPINAndVerify(t *testing.T) {
	h, err := HashPIN("123456")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	ok, err := VerifyPIN("123456", h)
	if err != nil || !ok {
		t.Fatalf("verify benar gagal: ok=%v err=%v", ok, err)
	}
	ok, _ = VerifyPIN("654321", h)
	if ok {
		t.Fatal("PIN salah harus gagal")
	}
}

func TestVerifyPINRejectsBadFormat(t *testing.T) {
	h, _ := HashPIN("123456")
	if _, err := VerifyPIN("abcdef", h); err == nil {
		t.Fatal("format PIN salah harus menghasilkan error, bukan mismatch")
	}
}

func TestGeneratePIN(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		p, err := GeneratePIN()
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != 6 || strings.HasPrefix(p, "-") {
			t.Fatalf("PIN tak terduga: %q", p)
		}
		seen[p] = true
	}
	if len(seen) < 5 {
		t.Fatalf("GeneratePIN terlalu sering menghasilkan nilai sama: %v", seen)
	}
}
