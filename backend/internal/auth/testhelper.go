package auth

import (
	"time"

	"fitcommerce/backend/internal/config"
)

// TestJWTConfig returns a JWTConfig suitable for unit tests.
func TestJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:        "test-access-secret-32bytes!!!!!",
		RefreshSecret: "test-refresh-secret-32bytes!!!!",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
	}
}
