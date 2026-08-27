// Command backend is the Go service for the Excalidraw collaborative canvas:
// auth + JWT issuance, in-memory room management, a pure WebSocket relay, and
// Firestore/Storage scene persistence. See docs/EDD.md for the architecture.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/config"
	"github.com/phil-blais/phil-canvas/backend/internal/firebaseapp"
	"github.com/phil-blais/phil-canvas/backend/internal/httpapi"
	"github.com/phil-blais/phil-canvas/backend/internal/relay"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/scenes"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	app, err := firebaseapp.New(ctx, cfg)
	if err != nil {
		log.Fatalf("firebase app: %v", err)
	}

	verifier, err := auth.NewFirebaseVerifier(ctx, app)
	if err != nil {
		log.Fatalf("firebase verifier: %v", err)
	}

	store, blobs, err := buildStores(ctx, cfg, app)
	if err != nil {
		log.Fatalf("scene stores: %v", err)
	}

	registry := rooms.NewRegistry()
	issuer := token.NewIssuer(cfg.JWTSecret, cfg.JWTTTL)
	authenticator := auth.NewAuthenticator(issuer)
	authHandler := auth.NewHandler(verifier, cfg.AdminAllowlist, issuer, registry)
	roomHandler := httpapi.NewRoomHandler(registry, authenticator)
	sceneHandler := httpapi.NewSceneHandler(store, blobs, registry, authenticator)
	relayHandler := relay.NewHandler(registry, issuer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	authHandler.Routes(mux)
	roomHandler.Routes(mux)
	sceneHandler.Routes(mux)
	relayHandler.Routes(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("backend listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("backend stopped")
}

// buildStores returns Firestore/Storage-backed scene stores when persistence is
// configured, or in-memory stores otherwise so the service can run for local
// frontend development without Firebase (scenes are then non-durable).
func buildStores(ctx context.Context, cfg *config.Config, app *firebase.App) (scenes.Store, scenes.BlobStore, error) {
	if !cfg.PersistenceEnabled() {
		log.Println("WARNING: Firebase persistence not configured — using in-memory scene store (non-durable)")
		return scenes.NewMemStore(), scenes.NewMemBlobStore(), nil
	}
	store, err := scenes.NewFirestoreStore(ctx, app)
	if err != nil {
		return nil, nil, err
	}
	blobs, err := scenes.NewGCSBlobStore(ctx, app, cfg.StorageBucket)
	if err != nil {
		return nil, nil, err
	}
	return store, blobs, nil
}
