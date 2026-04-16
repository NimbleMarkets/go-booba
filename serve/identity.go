//go:build !js

package serve

import "context"

// Identity is the minimal contract for authenticated subjects produced
// by layer-1 middleware. Future revisions may extend this interface
// additively (e.g., Claims, Roles); the v0.3 surface is intentionally
// just String() so any auth implementation can satisfy it cheaply.
type Identity interface {
	String() string
}

type identityCtxKey struct{}

// WithIdentity returns a derived context carrying id. Passing a nil
// identity returns ctx unchanged.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	if id == nil {
		return ctx
	}
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// IdentityFromContext returns the Identity attached to ctx, if any.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(Identity)
	return id, ok
}
