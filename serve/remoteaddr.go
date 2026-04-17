//go:build !js

package serve

import "context"

type remoteAddrCtxKey struct{}

// WithRemoteAddr returns a derived context carrying the client's
// remote address (typically r.RemoteAddr from the HTTP request that
// initiated the session). An empty addr returns ctx unchanged.
func WithRemoteAddr(ctx context.Context, addr string) context.Context {
	if addr == "" {
		return ctx
	}
	return context.WithValue(ctx, remoteAddrCtxKey{}, addr)
}

// RemoteAddrFromContext returns the remote address attached to ctx by
// the framework, or an empty string if none is present.
func RemoteAddrFromContext(ctx context.Context) string {
	s, _ := ctx.Value(remoteAddrCtxKey{}).(string)
	return s
}
