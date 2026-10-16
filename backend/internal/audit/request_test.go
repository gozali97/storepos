package audit

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRequestMetadataValidation(t *testing.T) {
	ctx := WithRequest(context.Background(), "127.0.0.1:1234", strings.Repeat("é", 600))
	m := ctx.Value(requestKey{}).(RequestMetadata)
	if m.IP != "127.0.0.1" || utf8.RuneCountInString(m.UserAgent) != 512 || !utf8.ValidString(m.UserAgent) {
		t.Fatal(m)
	}
	ctx = WithRequest(context.Background(), "spoofed-header", "agent")
	if m := ctx.Value(requestKey{}).(RequestMetadata); m.IP != "" {
		t.Fatal("invalid peer accepted", m)
	}
}
