// Package firebaseapp builds the shared Firebase Admin app used by auth,
// Firestore, and Storage, so credentials are resolved in one place.
package firebaseapp

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"

	"github.com/phil-blais/phil-canvas/backend/internal/config"
)

// New builds a Firebase Admin app. In emulator or in-memory dev mode it is built
// without credentials (emulators are selected by the standard *_EMULATOR_HOST
// env vars). In production it uses Application Default Credentials — the Cloud
// Run service account, or GOOGLE_APPLICATION_CREDENTIALS if set.
func New(ctx context.Context, cfg *config.Config) (*firebase.App, error) {
	var opts []option.ClientOption
	if cfg.WithoutCredentials() {
		opts = append(opts, option.WithoutAuthentication())
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.FirebaseProjectID}, opts...)
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}
	return app, nil
}
