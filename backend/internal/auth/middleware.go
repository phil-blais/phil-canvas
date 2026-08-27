package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
	"github.com/phil-blais/phil-canvas/backend/internal/web"
)

type contextKey int

const claimsKey contextKey = iota

// ErrForbidden indicates a valid token that lacks access to the target room.
var ErrForbidden = errors.New("forbidden")

// Authenticator validates Go-issued JWTs on protected routes.
type Authenticator struct {
	issuer *token.Issuer
}

// NewAuthenticator returns an Authenticator using the given issuer.
func NewAuthenticator(issuer *token.Issuer) *Authenticator {
	return &Authenticator{issuer: issuer}
}

// Middleware validates a Bearer token and stores its claims in the request
// context. Requests without a valid token are rejected with 401.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			web.WriteError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := a.issuer.Parse(raw)
		if err != nil {
			web.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFrom returns the validated claims stored by Middleware, if present.
func ClaimsFrom(ctx context.Context) (*token.Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*token.Claims)
	return claims, ok
}

// AuthorizeRoom enforces the role-asymmetric access rule for a room:
//   - guests must present a token bound to that exact room;
//   - any admin may join any live room. Documents are shared among admins, so
//     a room is not restricted to the admin who created it — AdminUID is
//     informational (who created it) rather than an access gate.
//
// It returns ErrForbidden when access is denied.
func AuthorizeRoom(reg *rooms.Registry, claims *token.Claims, roomID string) error {
	switch claims.Role {
	case token.RoleGuest:
		if claims.Room == roomID {
			return nil
		}
	case token.RoleAdmin:
		if _, ok := reg.Get(roomID); ok {
			return nil
		}
	}
	return ErrForbidden
}

// bearerToken extracts a token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	scheme, tok, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "bearer") || tok == "" {
		return "", false
	}
	return strings.TrimSpace(tok), true
}
