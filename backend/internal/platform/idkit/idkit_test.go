package idkit

import (
	"bytes"
	"testing"
	"time"
)

func TestNewV7HasVersionAndVariant(t *testing.T) {
	id := NewV7()
	if id.Version() != 7 {
		t.Errorf("version = %d, want 7", id.Version())
	}
	if id[8]&0xc0 != 0x80 {
		t.Errorf("variant bits = %08b", id[8]&0xc0)
	}
}

func TestNewV7TimestampMonotonicIsh(t *testing.T) {
	t0 := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 := t0.Add(2 * time.Millisecond)
	a := newV7At(t0)
	b := newV7At(t1)
	if bytes.Compare(a[:6], b[:6]) >= 0 {
		t.Errorf("timestamp a (%x) tidak < b (%x)", a[:6], b[:6])
	}
}

func TestStringRoundTrip(t *testing.T) {
	id := NewV7()
	s := id.String()
	got, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got != id {
		t.Fatalf("round trip failed: %v != %v", got, id)
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{
		"", "not-a-uuid",
		"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("Parse(%q) seharusnya error", c)
		}
	}
}
