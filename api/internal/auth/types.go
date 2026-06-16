package auth

// AdminLoginRequest is the request body for POST /auth/admin/login.
type AdminLoginRequest struct {
	Password string `json:"password"`
}

// AuthResponse is the response body for successful auth endpoints.
type AuthResponse struct {
	Authenticated bool     `json:"authenticated"`
	Identity      Identity `json:"identity,omitempty"`
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ConfigResponse is the response body for GET /auth/config.
type ConfigResponse struct {
	Enabled      bool       `json:"enabled"`
	AdminEnabled bool       `json:"adminEnabled"`
	OIDC         OIDCConfig `json:"oidc"`
}
