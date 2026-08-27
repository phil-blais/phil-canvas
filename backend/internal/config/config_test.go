package config

import "testing"

func TestPersistenceAndCredentialsModes(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		persistence bool
		noCreds     bool
	}{
		{"production (nothing set)", Config{}, true, false},
		{"in-memory dev", Config{DisablePersistence: true}, false, true},
		{"firestore emulator", Config{FirestoreEmulatorHost: "localhost:8081"}, true, true},
		{"auth emulator only", Config{AuthEmulatorHost: "localhost:9099"}, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.PersistenceEnabled(); got != tc.persistence {
				t.Errorf("PersistenceEnabled = %v, want %v", got, tc.persistence)
			}
			if got := tc.cfg.WithoutCredentials(); got != tc.noCreds {
				t.Errorf("WithoutCredentials = %v, want %v", got, tc.noCreds)
			}
		})
	}
}

func TestLoadHappyPath(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("FIREBASE_PROJECT_ID", "proj")
	t.Setenv("ADMIN_ALLOWLIST", "a@b.com, C@D.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if !cfg.PersistenceEnabled() {
		t.Error("persistence should be on by default")
	}
	if len(cfg.AdminAllowlist) != 2 {
		t.Errorf("allowlist size = %d, want 2", len(cfg.AdminAllowlist))
	}
	if _, ok := cfg.AdminAllowlist["c@d.com"]; !ok {
		t.Error("allowlist entries should be lowercased")
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("FIREBASE_PROJECT_ID", "proj")
	t.Setenv("ADMIN_ALLOWLIST", "a@b.com")
	t.Setenv("JWT_SECRET", "") // missing
	if _, err := Load(); err == nil {
		t.Fatal("expected error when JWT_SECRET is missing")
	}
}

func TestLoadParsesDisablePersistence(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("FIREBASE_PROJECT_ID", "proj")
	t.Setenv("ADMIN_ALLOWLIST", "a@b.com")
	t.Setenv("DISABLE_PERSISTENCE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PersistenceEnabled() {
		t.Error("DISABLE_PERSISTENCE=true should disable persistence")
	}
}
