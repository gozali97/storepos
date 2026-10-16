package audit

import (
	"context"
	"net"
	"strings"
)

type requestKey struct{}
type RequestMetadata struct{ IP, UserAgent string }

// WithRequest accepts direct peer IP only; proxy headers are not trusted.
func WithRequest(ctx context.Context, peer, userAgent string) context.Context {
	host, _, err := net.SplitHostPort(peer)
	if err != nil {
		host = peer
	}
	ip := net.ParseIP(host)
	host = ""
	if ip != nil {
		host = ip.String()
	}
	userAgent = strings.ToValidUTF8(userAgent, "")
	r := []rune(userAgent)
	if len(r) > 512 {
		userAgent = string(r[:512])
	}
	return context.WithValue(ctx, requestKey{}, RequestMetadata{IP: host, UserAgent: userAgent})
}
