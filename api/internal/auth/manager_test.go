package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)
	m, err := NewManager(context.Background(), slog.Default(), Config{
		Enabled:      true,
		SessionTTL:   time.Hour,
		SigningKey:   []byte("0123456789abcdef0123456789abcdef"),
		CookieSecure: false,
		Admin: AdminConfig{
			Enabled:      true,
			PasswordHash: string(hash),
		},
		Internal: InternalConfig{Token: "internal-token"},
	}, AllowAuthenticatedAuthorizer{})
	require.NoError(t, err)
	return m
}

func TestAdminLoginCreatesAuthenticatedSession(t *testing.T) {
	m := testManager(t)
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		require.True(t, ok)
		require.Equal(t, "admin", identity.Subject)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := m.Middleware(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/admin/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)
	require.Equal(t, http.StatusOK, loginResp.Code)

	protectedReq := httptest.NewRequest(http.MethodGet, "/apis/magosproject.io/v1alpha1/projects", nil)
	for _, cookie := range loginResp.Result().Cookies() {
		protectedReq.AddCookie(cookie)
	}
	protectedResp := httptest.NewRecorder()
	handler.ServeHTTP(protectedResp, protectedReq)
	require.Equal(t, http.StatusNoContent, protectedResp.Code)
}

func TestUnsafeUserRequestRequiresCSRFToken(t *testing.T) {
	m := testManager(t)
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	mux.HandleFunc("POST /apis/magosproject.io/v1alpha1/workspaces/default/demo/reconcile", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := m.Middleware(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/admin/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)
	require.Equal(t, http.StatusOK, loginResp.Code)

	req := httptest.NewRequest(http.MethodPost, "/apis/magosproject.io/v1alpha1/workspaces/default/demo/reconcile", nil)
	for _, cookie := range loginResp.Result().Cookies() {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)

	for _, cookie := range loginResp.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNoContent, resp.Code)
}

func TestLogoutRequiresCSRFTokenWhenSessionCookieIsPresent(t *testing.T) {
	m := testManager(t)
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	handler := m.Middleware(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/admin/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)
	require.Equal(t, http.StatusOK, loginResp.Code)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	for _, cookie := range loginResp.Result().Cookies() {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)

	for _, cookie := range loginResp.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNoContent, resp.Code)
	requireLogoutClearsCookie(t, resp.Result().Cookies(), sessionCookieName)
	requireLogoutClearsCookie(t, resp.Result().Cookies(), csrfCookieName)
}

func TestLogoutWithoutSessionCookieDoesNotClearSession(t *testing.T) {
	m := testManager(t)
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	handler := m.Middleware(mux)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Empty(t, resp.Result().Cookies())
}

func TestInternalRouteRequiresServiceToken(t *testing.T) {
	m := testManager(t)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /internal/apis/magosproject.io/v1alpha1/workspaces/default/demo/runs/run-1", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		require.True(t, ok)
		require.Equal(t, ProviderService, identity.Provider)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := m.Middleware(mux)

	req := httptest.NewRequest(http.MethodPut, "/internal/apis/magosproject.io/v1alpha1/workspaces/default/demo/runs/run-1", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusUnauthorized, resp.Code)

	req = httptest.NewRequest(http.MethodPut, "/internal/apis/magosproject.io/v1alpha1/workspaces/default/demo/runs/run-1", nil)
	req.Header.Set("Authorization", "Bearer internal-token")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNoContent, resp.Code)
}

func requireLogoutClearsCookie(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			require.Equal(t, -1, cookie.MaxAge)
			require.Empty(t, cookie.Value)
			return
		}
	}
	require.Failf(t, "missing cookie", "expected %q to be cleared", name)
}
