package auth

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSessionTTL    = 24 * time.Hour
	defaultUsernameClaim = "email"
	defaultGroupsClaim   = "groups"
)

// Config controls authentication for the API server.
type Config struct {
	Enabled        bool
	BaseURL        string
	CookieSecure   bool
	AllowedOrigins []string
	SessionTTL     time.Duration
	SigningKey     []byte
	Admin          AdminConfig
	OIDC           OIDCConfig
	Internal       InternalConfig
}

type AdminConfig struct {
	Enabled      bool
	PasswordHash string
}

type OIDCConfig struct {
	Enabled          bool     `json:"enabled"`
	IssuerURL        string   `json:"issuerUrl,omitempty"`
	ClientID         string   `json:"clientId,omitempty"`
	ClientSecret     string   `json:"-"`
	UsernameClaim    string   `json:"usernameClaim,omitempty"`
	GroupsClaim      string   `json:"groupsClaim,omitempty"`
	AdditionalScopes []string `json:"additionalScopes,omitempty"`
	LoginURL         string   `json:"loginUrl,omitempty"`
}

type InternalConfig struct {
	Token string
}

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		Enabled:        parseBoolEnv("MAGOS_AUTH_ENABLED"),
		BaseURL:        strings.TrimRight(os.Getenv("MAGOS_AUTH_BASE_URL"), "/"),
		CookieSecure:   parseBoolEnv("MAGOS_AUTH_COOKIE_SECURE"),
		AllowedOrigins: splitCSV(os.Getenv("MAGOS_AUTH_ALLOWED_ORIGINS")),
		SessionTTL:     defaultSessionTTL,
		SigningKey:     []byte(os.Getenv("MAGOS_AUTH_SESSION_SIGNING_KEY")),
		Admin: AdminConfig{
			Enabled:      parseBoolEnv("MAGOS_AUTH_ADMIN_ENABLED"),
			PasswordHash: os.Getenv("MAGOS_AUTH_ADMIN_PASSWORD_HASH"),
		},
		OIDC: OIDCConfig{
			Enabled:          parseBoolEnv("MAGOS_AUTH_OIDC_ENABLED"),
			IssuerURL:        strings.TrimRight(os.Getenv("MAGOS_AUTH_OIDC_ISSUER_URL"), "/"),
			ClientID:         os.Getenv("MAGOS_AUTH_OIDC_CLIENT_ID"),
			ClientSecret:     os.Getenv("MAGOS_AUTH_OIDC_CLIENT_SECRET"),
			UsernameClaim:    firstNonEmpty(os.Getenv("MAGOS_AUTH_OIDC_USERNAME_CLAIM"), defaultUsernameClaim),
			GroupsClaim:      firstNonEmpty(os.Getenv("MAGOS_AUTH_OIDC_GROUPS_CLAIM"), defaultGroupsClaim),
			AdditionalScopes: splitCSV(os.Getenv("MAGOS_AUTH_OIDC_ADDITIONAL_SCOPES")),
		},
		Internal: InternalConfig{
			Token: os.Getenv("MAGOS_AUTH_INTERNAL_TOKEN"),
		},
	}

	if raw := os.Getenv("MAGOS_AUTH_SESSION_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MAGOS_AUTH_SESSION_TTL %q is invalid: %w", raw, err)
		}
		cfg.SessionTTL = ttl
	}

	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if len(c.SigningKey) < 32 {
		return fmt.Errorf("MAGOS_AUTH_SESSION_SIGNING_KEY must be at least 32 bytes when auth is enabled")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("MAGOS_AUTH_SESSION_TTL must be positive")
	}
	if c.Admin.Enabled && c.Admin.PasswordHash == "" {
		return fmt.Errorf("MAGOS_AUTH_ADMIN_PASSWORD_HASH is required when admin auth is enabled")
	}
	if c.OIDC.Enabled {
		if c.OIDC.IssuerURL == "" {
			return fmt.Errorf("MAGOS_AUTH_OIDC_ISSUER_URL is required when OIDC is enabled")
		}
		if _, err := url.ParseRequestURI(c.OIDC.IssuerURL); err != nil {
			return fmt.Errorf("MAGOS_AUTH_OIDC_ISSUER_URL %q is invalid: %w", c.OIDC.IssuerURL, err)
		}
		if c.OIDC.ClientID == "" {
			return fmt.Errorf("MAGOS_AUTH_OIDC_CLIENT_ID is required when OIDC is enabled")
		}
		if c.OIDC.UsernameClaim == "" {
			return fmt.Errorf("MAGOS_AUTH_OIDC_USERNAME_CLAIM must not be empty")
		}
	}
	return nil
}

func parseBoolEnv(name string) bool {
	v, err := strconv.ParseBool(os.Getenv(name))
	if err != nil {
		return false
	}
	return v
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
