package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

type oidcStateClaims struct {
	RedirectTo   string `json:"redirectTo,omitempty"`
	State        string `json:"state"`
	CodeVerifier string `json:"codeVerifier"`
	jwt.RegisteredClaims
}

func (m *Manager) oidcOAuthConfig() oauth2.Config {
	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	scopes = append(scopes, m.cfg.OIDC.AdditionalScopes...)
	return oauth2.Config{
		ClientID:     m.cfg.OIDC.ClientID,
		ClientSecret: m.cfg.OIDC.ClientSecret,
		Endpoint:     m.provider.Endpoint(),
		RedirectURL:  m.baseURL() + "/auth/oidc/callback",
		Scopes:       scopes,
	}
}

// startOIDC godoc
//
//	@Summary	Initiate OIDC login flow
//	@Tags		Auth
//	@Param		redirectTo	query	string	false	"Path to redirect to after login"
//	@Success	302
//	@Failure	404	{object}	ErrorResponse
//	@Failure	503	{object}	ErrorResponse
//	@Router		/auth/oidc/login [get]
func (m *Manager) startOIDC(w http.ResponseWriter, r *http.Request) {
	if !m.cfg.Enabled || !m.cfg.OIDC.Enabled {
		writeError(w, http.StatusNotFound, "oidc is not enabled")
		return
	}
	if err := m.ensureOIDC(r.Context()); err != nil {
		m.logger.Error("oidc provider initialization failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "oidc provider is unavailable")
		return
	}

	state, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start oidc login")
		return
	}
	verifier, err := randomToken(64)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start oidc login")
		return
	}

	redirectTo := safeRedirectPath(r.URL.Query().Get("redirectTo"))
	now := time.Now().UTC()
	stateClaims := oidcStateClaims{
		RedirectTo:   redirectTo,
		State:        state,
		CodeVerifier: verifier,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer(),
			Audience:  []string{m.issuer()},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
	}
	signedState, err := jwt.NewWithClaims(jwt.SigningMethodHS256, stateClaims).SignedString(m.cfg.SigningKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start oidc login")
		return
	}
	http.SetCookie(w, m.cookie(stateCookieName, signedState, true, 10*time.Minute))

	challenge := pkceChallenge(verifier)
	oauthConfig := m.oidcOAuthConfig()
	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// completeOIDC godoc
//
//	@Summary	OIDC callback – completes the login flow
//	@Tags		Auth
//	@Param		code	query	string	true	"Authorization code"
//	@Param		state	query	string	true	"OAuth2 state"
//	@Success	302
//	@Failure	400	{object}	ErrorResponse
//	@Failure	401	{object}	ErrorResponse
//	@Router		/auth/oidc/callback [get]
func (m *Manager) completeOIDC(w http.ResponseWriter, r *http.Request) {
	if !m.cfg.Enabled || !m.cfg.OIDC.Enabled {
		writeError(w, http.StatusNotFound, "oidc is not enabled")
		return
	}
	if err := m.ensureOIDC(r.Context()); err != nil {
		m.logger.Error("oidc provider initialization failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "oidc provider is unavailable")
		return
	}
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" {
		writeError(w, http.StatusBadRequest, "missing oidc state")
		return
	}
	stateClaims := oidcStateClaims{}
	token, err := jwt.ParseWithClaims(stateCookie.Value, &stateClaims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Header["alg"])
		}
		return m.cfg.SigningKey, nil
	}, jwt.WithIssuer(m.issuer()), jwt.WithAudience(m.issuer()))
	if err != nil || !token.Valid || stateClaims.State == "" || stateClaims.CodeVerifier == "" {
		writeError(w, http.StatusBadRequest, "invalid oidc state")
		return
	}
	http.SetCookie(w, m.expiredCookie(stateCookieName, true))

	if r.URL.Query().Get("state") != stateClaims.State {
		writeError(w, http.StatusBadRequest, "invalid oidc state")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing oidc code")
		return
	}

	oauthConfig := m.oidcOAuthConfig()
	oauthToken, err := oauthConfig.Exchange(
		r.Context(),
		code,
		oauth2.SetAuthURLParam("code_verifier", stateClaims.CodeVerifier),
	)
	if err != nil {
		m.logger.Error("oidc token exchange failed", "error", err)
		writeError(w, http.StatusUnauthorized, "oidc token exchange failed")
		return
	}
	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		writeError(w, http.StatusUnauthorized, "oidc response did not include an id token")
		return
	}

	idToken, err := m.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		m.logger.Error("oidc id token verification failed", "error", err)
		writeError(w, http.StatusUnauthorized, "oidc token verification failed")
		return
	}

	identity, err := m.identityFromOIDCToken(r.Context(), idToken)
	if err != nil {
		m.logger.Error("oidc claim extraction failed", "error", err)
		writeError(w, http.StatusUnauthorized, "oidc claims are invalid")
		return
	}
	if err := m.setSession(w, identity); err != nil {
		m.logger.Error("failed to set oidc session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	redirectTo := stateClaims.RedirectTo
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (m *Manager) identityFromOIDCToken(_ context.Context, token *oidc.IDToken) (Identity, error) {
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return Identity{}, err
	}
	username := claimString(claims, m.cfg.OIDC.UsernameClaim)
	if username == "" {
		username = token.Subject
	}
	return Identity{
		Provider: ProviderOIDC,
		Subject:  token.Subject,
		Username: username,
		Groups:   claimStringSlice(claims, m.cfg.OIDC.GroupsClaim),
		Claims:   claims,
		IsAdmin:  false,
	}, nil
}

func (m *Manager) initOIDC(ctx context.Context) error {
	if !m.cfg.Enabled || !m.cfg.OIDC.Enabled {
		return nil
	}
	provider, err := oidc.NewProvider(ctx, m.cfg.OIDC.IssuerURL)
	if err != nil {
		return fmt.Errorf("initialize oidc provider: %w", err)
	}
	m.provider = provider
	m.verifier = provider.Verifier(&oidc.Config{ClientID: m.cfg.OIDC.ClientID})
	return nil
}

func (m *Manager) ensureOIDC(ctx context.Context) error {
	if m.provider != nil && m.verifier != nil {
		return nil
	}
	m.oidcMu.Lock()
	defer m.oidcMu.Unlock()
	if m.provider != nil && m.verifier != nil {
		return nil
	}
	return m.initOIDC(ctx)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func claimString(claims map[string]any, name string) string {
	if name == "" {
		return ""
	}
	v, ok := claims[name]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		slog.Warn("oidc claim is not a string, coercing", "claim", name, "type", fmt.Sprintf("%T", v))
		return fmt.Sprint(v)
	}
	return s
}

func claimStringSlice(claims map[string]any, name string) []string {
	if name == "" {
		return nil
	}
	v, ok := claims[name]
	if !ok {
		return nil
	}
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func safeRedirectPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return ""
	}
	return raw
}
