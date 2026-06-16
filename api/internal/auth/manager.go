package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
)

type Manager struct {
	cfg        Config
	logger     *slog.Logger
	authorizer Authorizer
	oidcMu     sync.Mutex
	provider   *oidc.Provider
	verifier   *oidc.IDTokenVerifier
}

func NewManager(ctx context.Context, logger *slog.Logger, cfg Config, authorizer Authorizer) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if authorizer == nil {
		authorizer = AllowAuthenticatedAuthorizer{}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	m := &Manager{cfg: cfg, logger: logger, authorizer: authorizer}
	if err := m.initOIDC(ctx); err != nil {
		logger.Error("failed to initialize oidc provider; will retry on login", "error", err)
	}
	return m, nil
}

func (m *Manager) Enabled() bool {
	return m != nil && m.cfg.Enabled
}

func (m *Manager) AllowsOrigin(origin string) bool {
	if m == nil || origin == "" {
		return false
	}
	for _, allowed := range m.cfg.AllowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/config", m.config)
	mux.HandleFunc("GET /auth/me", m.me)
	mux.HandleFunc("POST /auth/admin/login", m.adminLogin)
	mux.HandleFunc("POST /auth/logout", m.logout)
	mux.HandleFunc("GET /auth/oidc/login", m.startOIDC)
	mux.HandleFunc("GET /auth/oidc/callback", m.completeOIDC)
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	if !m.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/internal/apis/") {
			m.authenticateInternal(next, w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/apis/") {
			m.authenticateUser(next, w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// config godoc
//
//	@Summary	Get authentication configuration
//	@Tags		Auth
//	@Produce	json
//	@Success	200	{object}	ConfigResponse
//	@Router		/auth/config [get]
func (m *Manager) config(w http.ResponseWriter, _ *http.Request) {
	resp := ConfigResponse{
		Enabled:      m.cfg.Enabled,
		AdminEnabled: m.cfg.Enabled && m.cfg.Admin.Enabled,
		OIDC: OIDCConfig{
			Enabled:          m.cfg.Enabled && m.cfg.OIDC.Enabled,
			IssuerURL:        m.cfg.OIDC.IssuerURL,
			ClientID:         m.cfg.OIDC.ClientID,
			UsernameClaim:    m.cfg.OIDC.UsernameClaim,
			GroupsClaim:      m.cfg.OIDC.GroupsClaim,
			AdditionalScopes: m.cfg.OIDC.AdditionalScopes,
			LoginURL:         "/auth/oidc/login",
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// me godoc
//
//	@Summary	Get current authenticated identity
//	@Tags		Auth
//	@Security	CookieAuth
//	@Produce	json
//	@Success	200	{object}	AuthResponse
//	@Failure	401	{object}	ErrorResponse
//	@Router		/auth/me [get]
func (m *Manager) me(w http.ResponseWriter, r *http.Request) {
	if !m.cfg.Enabled {
		writeJSON(w, http.StatusOK, AuthResponse{})
		return
	}
	identity, ok := m.identityFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, AuthResponse{Authenticated: true, Identity: identity})
}

// adminLogin godoc
//
//	@Summary	Login with admin password
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		AdminLoginRequest	true	"Login credentials"
//	@Success	200		{object}	AuthResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	403		{object}	ErrorResponse
//	@Router		/auth/admin/login [post]
func (m *Manager) adminLogin(w http.ResponseWriter, r *http.Request) {
	if !m.cfg.Enabled || !m.cfg.Admin.Enabled {
		writeError(w, http.StatusForbidden, "admin login is not enabled")
		return
	}
	var req AdminLoginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.cfg.Admin.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusForbidden, "invalid password")
		return
	}
	identity := Identity{
		Provider: ProviderAdmin,
		Subject:  "admin",
		Username: "admin",
		IsAdmin:  true,
	}
	if err := m.setSession(w, identity); err != nil {
		m.logger.Error("failed to create admin session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	writeJSON(w, http.StatusOK, AuthResponse{Authenticated: true, Identity: identity})
}

// logout godoc
//
//	@Summary	Logout and clear session
//	@Tags		Auth
//	@Success	204
//	@Failure	200	{object}	ErrorResponse
//	@Router		/auth/logout [post]
func (m *Manager) logout(w http.ResponseWriter, _ *http.Request) {
	m.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (m *Manager) authenticateInternal(next http.Handler, w http.ResponseWriter, r *http.Request) {
	if m.cfg.Internal.Token == "" {
		writeError(w, http.StatusUnauthorized, "internal authentication is not configured")
		return
	}
	token := bearerToken(r)
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(m.cfg.Internal.Token)) != 1 {
		writeError(w, http.StatusUnauthorized, "internal authentication required")
		return
	}
	identity := Identity{Provider: ProviderService, Subject: "workspace-controller", Username: "workspace-controller"}
	ctx := ContextWithIdentity(r.Context(), identity)
	if err := m.authorizer.Authorize(ctx, identity, operationFromRequest(r, true)); err != nil {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	next.ServeHTTP(w, r.WithContext(ctx))
}

func (m *Manager) authenticateUser(next http.Handler, w http.ResponseWriter, r *http.Request) {
	identity, ok := m.identityFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if unsafeMethod(r.Method) && !m.csrfValid(r) {
		writeError(w, http.StatusForbidden, "csrf token is required")
		return
	}
	ctx := ContextWithIdentity(r.Context(), identity)
	if err := m.authorizer.Authorize(ctx, identity, operationFromRequest(r, false)); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	next.ServeHTTP(w, r.WithContext(ctx))
}

func (m *Manager) isPublic(r *http.Request) bool {
	switch r.URL.Path {
	case "/healthz", "/readyz", "/openapi.json", "/docs", "/auth/config", "/auth/admin/login", "/auth/logout", "/auth/me", "/auth/oidc/login", "/auth/oidc/callback":
		return true
	default:
		return false
	}
}

func (m *Manager) issuer() string {
	if m.cfg.BaseURL != "" {
		return m.cfg.BaseURL
	}
	return "magos"
}

func (m *Manager) audience() string {
	if m.cfg.BaseURL != "" {
		return m.cfg.BaseURL
	}
	return "magos"
}

func (m *Manager) baseURL() string {
	if m.cfg.BaseURL != "" {
		return m.cfg.BaseURL
	}
	return "http://localhost:8080"
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func operationFromRequest(r *http.Request, internal bool) Operation {
	return Operation{
		Method:    r.Method,
		Path:      r.URL.Path,
		Resource:  resourceFromPath(r.URL.Path),
		Namespace: r.PathValue("namespace"),
		Name:      r.PathValue("name"),
		Internal:  internal,
	}
}

func resourceFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "v1alpha1" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
