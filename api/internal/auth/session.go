package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	sessionCookieName = "magos_session"
	csrfCookieName    = "magos_csrf"
	stateCookieName   = "magos_oidc_state"
)

type sessionClaims struct {
	Provider string         `json:"provider"`
	Username string         `json:"username,omitempty"`
	Groups   []string       `json:"groups,omitempty"`
	Claims   map[string]any `json:"claims,omitempty"`
	IsAdmin  bool           `json:"isAdmin"`
	jwt.RegisteredClaims
}

func (m *Manager) setSession(w http.ResponseWriter, identity Identity) error {
	now := time.Now().UTC()
	claims := sessionClaims{
		Provider: string(identity.Provider),
		Username: identity.Username,
		Groups:   identity.Groups,
		Claims:   identity.Claims,
		IsAdmin:  identity.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   identity.Subject,
			Issuer:    m.issuer(),
			Audience:  []string{m.audience()},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.cfg.SessionTTL)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.cfg.SigningKey)
	if err != nil {
		return fmt.Errorf("sign session: %w", err)
	}
	http.SetCookie(w, m.cookie(sessionCookieName, token, true, m.cfg.SessionTTL))

	csrf, err := randomToken(32)
	if err != nil {
		return fmt.Errorf("generate csrf token: %w", err)
	}
	http.SetCookie(w, m.cookie(csrfCookieName, csrf, false, m.cfg.SessionTTL))
	return nil
}

func (m *Manager) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, m.expiredCookie(sessionCookieName, true))
	http.SetCookie(w, m.expiredCookie(csrfCookieName, false))
}

func (m *Manager) identityFromRequest(r *http.Request) (Identity, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return Identity{}, false
	}
	claims := sessionClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Header["alg"])
		}
		return m.cfg.SigningKey, nil
	}, jwt.WithIssuer(m.issuer()), jwt.WithAudience(m.audience()))
	if err != nil || !token.Valid {
		return Identity{}, false
	}
	return Identity{
		Provider: Provider(claims.Provider),
		Subject:  claims.Subject,
		Username: claims.Username,
		Groups:   claims.Groups,
		Claims:   claims.Claims,
		IsAdmin:  claims.IsAdmin,
	}, true
}

func (m *Manager) csrfValid(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) == 1
}

func (m *Manager) cookie(name, value string, httpOnly bool, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: httpOnly,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (m *Manager) expiredCookie(name string, httpOnly bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: httpOnly,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func randomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
