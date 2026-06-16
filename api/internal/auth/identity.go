package auth

import "context"

type Provider string

const (
	ProviderAdmin   Provider = "admin"
	ProviderOIDC    Provider = "oidc"
	ProviderService Provider = "service"
)

// Identity is intentionally richer than the first authentication feature needs.
// RBAC can consume this stable shape later without changing handlers again.
type Identity struct {
	Provider Provider       `json:"provider"`
	Subject  string         `json:"subject"`
	Username string         `json:"username,omitempty"`
	Groups   []string       `json:"groups,omitempty"`
	Claims   map[string]any `json:"claims,omitempty"`
	IsAdmin  bool           `json:"isAdmin"`
}

type contextKey struct{}

func ContextWithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}

type Operation struct {
	Method    string
	Path      string
	Resource  string
	Namespace string
	Name      string
	Internal  bool
}

type Authorizer interface {
	Authorize(context.Context, Identity, Operation) error
}

type AllowAuthenticatedAuthorizer struct{}

func (AllowAuthenticatedAuthorizer) Authorize(context.Context, Identity, Operation) error {
	return nil
}
