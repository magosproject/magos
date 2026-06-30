package auth

import (
	"context"
	"errors"
)

// ErrUnauthenticated is returned by an Authorizer when the identity cannot be
// verified and the caller should re-authenticate (HTTP 401).
var ErrUnauthenticated = errors.New("authentication required")

// ErrForbidden is returned by an Authorizer when the identity is valid but
// does not have permission to perform the operation (HTTP 403).
var ErrForbidden = errors.New("forbidden")

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
	// Method is the raw HTTP method.
	Method string
	// Verb is the normalized RBAC verb derived from the HTTP method and path:
	// get, list, create, update, patch, delete, watch.
	Verb      string
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
