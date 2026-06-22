package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateRequiresInteractiveProviderWhenAuthIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "auth disabled allows no providers",
			cfg: Config{
				Enabled:    false,
				SessionTTL: time.Hour,
			},
		},
		{
			name: "auth enabled requires provider",
			cfg: Config{
				Enabled:    true,
				SessionTTL: time.Hour,
				SigningKey: []byte("0123456789abcdef0123456789abcdef"),
			},
			wantErr: "at least one interactive auth provider must be enabled when auth is enabled",
		},
		{
			name: "admin provider",
			cfg: Config{
				Enabled:    true,
				SessionTTL: time.Hour,
				SigningKey: []byte("0123456789abcdef0123456789abcdef"),
				Admin: AdminConfig{
					Enabled:      true,
					PasswordHash: "hash",
				},
			},
		},
		{
			name: "oidc provider",
			cfg: Config{
				Enabled:    true,
				SessionTTL: time.Hour,
				SigningKey: []byte("0123456789abcdef0123456789abcdef"),
				OIDC: OIDCConfig{
					Enabled:       true,
					IssuerURL:     "https://issuer.example.test",
					ClientID:      "client-id",
					UsernameClaim: "email",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}
