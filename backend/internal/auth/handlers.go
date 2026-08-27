package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
	"github.com/phil-blais/phil-canvas/backend/internal/web"
)

// Handler serves the public auth endpoints.
type Handler struct {
	verifier  IDTokenVerifier
	allowlist map[string]struct{}
	issuer    *token.Issuer
	rooms     *rooms.Registry
	limiter   *rateLimiter
}

// NewHandler builds an auth Handler. The allowlist set must contain lowercased
// emails and/or UIDs.
func NewHandler(v IDTokenVerifier, allowlist map[string]struct{}, issuer *token.Issuer, reg *rooms.Registry) *Handler {
	return &Handler{
		verifier:  v,
		allowlist: allowlist,
		issuer:    issuer,
		rooms:     reg,
		// Cap guest code attempts: 10 per minute per IP.
		limiter: newRateLimiter(10, time.Minute),
	}
}

// Routes registers the auth endpoints on the given mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/admin", h.AdminLogin)
	mux.HandleFunc("POST /auth/guest", h.GuestLogin)
}

type adminLoginRequest struct {
	IDToken string `json:"idToken"`
}

type tokenResponse struct {
	Token string `json:"token"`
	Room  string `json:"room,omitempty"`
}

// AdminLogin verifies a Firebase ID token, enforces the admin allowlist, and
// issues an admin JWT. A verified token alone is not authorization.
func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}
	if req.IDToken == "" {
		web.WriteError(w, http.StatusBadRequest, "idToken is required")
		return
	}

	user, err := h.verifier.Verify(r.Context(), req.IDToken)
	if err != nil {
		web.WriteError(w, http.StatusUnauthorized, "invalid Firebase ID token")
		return
	}
	if !h.isAllowed(user) {
		web.WriteError(w, http.StatusForbidden, "not authorized as admin")
		return
	}

	tok, err := h.issuer.IssueAdmin(user.UID, displayName(user))
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	web.WriteJSON(w, http.StatusOK, tokenResponse{Token: tok})
}

// displayName resolves the best human-readable identifier for a verified
// user, for attributing "saved by" on scene versions — a display name if
// Firebase has one, else the email, else the UID (never blank).
func displayName(user *VerifiedUser) string {
	if user.Name != "" {
		return user.Name
	}
	if user.Email != "" {
		return user.Email
	}
	return user.UID
}

// isAllowed reports whether the verified user is on the admin allowlist, by
// either email or UID (both matched case-insensitively).
func (h *Handler) isAllowed(user *VerifiedUser) bool {
	if _, ok := h.allowlist[strings.ToLower(user.UID)]; ok {
		return true
	}
	if user.Email != "" {
		if _, ok := h.allowlist[strings.ToLower(user.Email)]; ok {
			return true
		}
	}
	return false
}

type guestLoginRequest struct {
	Room string `json:"room"`
	Code string `json:"code"`
}

// GuestLogin validates a room's 4-char code and issues a room-bound guest JWT.
// Failures are rate-limited per IP and return a uniform error so a missing room
// and a wrong code are indistinguishable.
func (h *Handler) GuestLogin(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(clientIP(r)) {
		web.WriteError(w, http.StatusTooManyRequests, "too many attempts, try again later")
		return
	}

	var req guestLoginRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}

	room, ok := h.rooms.Get(req.Room)
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	// Compare in constant time; only treat as valid when the room exists.
	valid := ok && subtle.ConstantTimeCompare([]byte(code), []byte(room.Code)) == 1
	if !valid {
		web.WriteError(w, http.StatusUnauthorized, "invalid room or code")
		return
	}

	tok, err := h.issuer.IssueGuest(newGuestUID(), room.ID)
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	web.WriteJSON(w, http.StatusOK, tokenResponse{Token: tok, Room: room.ID})
}

// newGuestUID returns a random opaque identifier for a guest (guests have no
// Firebase account).
func newGuestUID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "guest-" + hex.EncodeToString(b[:])
}

// clientIP extracts the caller's IP, honoring the first X-Forwarded-For hop set
// by the Firebase Hosting → Cloud Run proxy in production.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
