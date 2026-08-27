// Package config loads and validates service configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	Port              string
	JWTSecret         []byte
	JWTTTL            time.Duration
	FirebaseProjectID string
	StorageBucket     string
	// AdminAllowlist is the set of lowercased emails and UIDs permitted to
	// receive an admin JWT. A verified Google token is not sufficient on its own.
	AdminAllowlist map[string]struct{}
	// Emulator hosts mirror the standard *_EMULATOR_HOST env vars; any non-empty
	// value means we are running against local emulators (no GCP credentials).
	AuthEmulatorHost      string
	FirestoreEmulatorHost string
	StorageEmulatorHost   string
	// DisablePersistence (DISABLE_PERSISTENCE=true) forces the in-memory scene
	// store, for local frontend development without any Firebase. Persistence is
	// on by default so production (Cloud Run) uses Firestore/Storage.
	DisablePersistence bool
}

// PersistenceEnabled reports whether Firestore/Storage-backed persistence is
// used. On by default; disabled only for explicit in-memory dev mode.
func (c *Config) PersistenceEnabled() bool {
	return !c.DisablePersistence
}

// usingEmulators reports whether any local emulator is targeted.
func (c *Config) usingEmulators() bool {
	return c.AuthEmulatorHost != "" || c.FirestoreEmulatorHost != "" || c.StorageEmulatorHost != ""
}

// WithoutCredentials reports whether the Firebase app should be built without
// credentials. True for emulator or in-memory dev mode; false in production,
// where Application Default Credentials (the Cloud Run service account, or
// GOOGLE_APPLICATION_CREDENTIALS if set) are used.
func (c *Config) WithoutCredentials() bool {
	return c.DisablePersistence || c.usingEmulators()
}

// Load reads configuration from environment variables, applying defaults and
// validating required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              envOr("PORT", "8080"),
		JWTSecret:         []byte(os.Getenv("JWT_SECRET")),
		JWTTTL:            12 * time.Hour,
		FirebaseProjectID: os.Getenv("FIREBASE_PROJECT_ID"),
		StorageBucket:     os.Getenv("STORAGE_BUCKET"),
		AdminAllowlist:    parseAllowlist(os.Getenv("ADMIN_ALLOWLIST")),

		AuthEmulatorHost:      os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"),
		FirestoreEmulatorHost: os.Getenv("FIRESTORE_EMULATOR_HOST"),
		StorageEmulatorHost:   os.Getenv("STORAGE_EMULATOR_HOST"),
		DisablePersistence:    os.Getenv("DISABLE_PERSISTENCE") == "true",
	}

	if ttl := os.Getenv("JWT_TTL"); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_TTL %q: %w", ttl, err)
		}
		cfg.JWTTTL = d
	}

	if len(cfg.JWTSecret) == 0 {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.FirebaseProjectID == "" {
		return nil, fmt.Errorf("FIREBASE_PROJECT_ID is required")
	}
	if len(cfg.AdminAllowlist) == 0 {
		return nil, fmt.Errorf("ADMIN_ALLOWLIST is required (comma-separated admin emails or UIDs)")
	}

	return cfg, nil
}

// parseAllowlist splits a comma-separated list into a set of lowercased,
// trimmed entries, ignoring blanks.
func parseAllowlist(raw string) map[string]struct{} {
	set := make(map[string]struct{})
	for entry := range strings.SplitSeq(raw, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry != "" {
			set[entry] = struct{}{}
		}
	}
	return set
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
