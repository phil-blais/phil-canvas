package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

type fakeVerifier struct {
	user *VerifiedUser
	err  error
}

func (f fakeVerifier) Verify(context.Context, string) (*VerifiedUser, error) {
	return f.user, f.err
}

func newTestHandler(v IDTokenVerifier, reg *rooms.Registry) (*Handler, *token.Issuer) {
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	allowlist := map[string]struct{}{"admin@example.com": {}}
	return NewHandler(v, allowlist, iss, reg), iss
}

func post(h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func decodeToken(t *testing.T, rr *httptest.ResponseRecorder) tokenResponse {
	t.Helper()
	var resp tokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rr.Body.String())
	}
	return resp
}

func TestAdminLoginAllowlisted(t *testing.T) {
	v := fakeVerifier{user: &VerifiedUser{UID: "uid-1", Email: "admin@example.com"}}
	h, iss := newTestHandler(v, rooms.NewRegistry())

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	claims, err := iss.Parse(decodeToken(t, rr).Token)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if claims.Role != token.RoleAdmin || claims.Subject != "uid-1" {
		t.Errorf("claims = %+v, want admin/uid-1", claims)
	}
}

func TestAdminLoginPrefersDisplayNameOverEmailAndUID(t *testing.T) {
	v := fakeVerifier{user: &VerifiedUser{UID: "uid-1", Email: "admin@example.com", Name: "Ada Lovelace"}}
	h, iss := newTestHandler(v, rooms.NewRegistry())

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	claims, err := iss.Parse(decodeToken(t, rr).Token)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if claims.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want Ada Lovelace", claims.Name)
	}
}

func TestAdminLoginFallsBackToEmailWhenNoDisplayName(t *testing.T) {
	v := fakeVerifier{user: &VerifiedUser{UID: "uid-1", Email: "admin@example.com", Name: ""}}
	h, iss := newTestHandler(v, rooms.NewRegistry())

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	claims, err := iss.Parse(decodeToken(t, rr).Token)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if claims.Name != "admin@example.com" {
		t.Errorf("name = %q, want admin@example.com (email fallback)", claims.Name)
	}
}

func TestAdminLoginFallsBackToUIDWhenNoDisplayNameOrEmail(t *testing.T) {
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	h := NewHandler(
		fakeVerifier{user: &VerifiedUser{UID: "uid-xyz", Email: "", Name: ""}},
		map[string]struct{}{"uid-xyz": {}},
		iss, rooms.NewRegistry(),
	)

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	claims, err := iss.Parse(decodeToken(t, rr).Token)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if claims.Name != "uid-xyz" {
		t.Errorf("name = %q, want uid-xyz (UID fallback)", claims.Name)
	}
}

func TestAdminLoginNotAllowlistedIsForbidden(t *testing.T) {
	v := fakeVerifier{user: &VerifiedUser{UID: "uid-2", Email: "stranger@example.com"}}
	h, _ := newTestHandler(v, rooms.NewRegistry())

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestAdminLoginInvalidTokenIsUnauthorized(t *testing.T) {
	v := fakeVerifier{err: errors.New("bad token")}
	h, _ := newTestHandler(v, rooms.NewRegistry())

	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"bad"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAdminLoginMissingTokenIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(fakeVerifier{}, rooms.NewRegistry())
	rr := post(h.AdminLogin, "/auth/admin", `{}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestAdminLoginAllowlistByUID(t *testing.T) {
	// Allowlist entry is a UID rather than an email.
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	h := NewHandler(
		fakeVerifier{user: &VerifiedUser{UID: "uid-xyz", Email: ""}},
		map[string]struct{}{"uid-xyz": {}},
		iss, rooms.NewRegistry(),
	)
	rr := post(h.AdminLogin, "/auth/admin", `{"idToken":"good"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func newRoom(code string) *rooms.Room {
	return rooms.NewRoom("room-1", "swift-river-42", code, "uid-1", "")
}

func TestGuestLoginValidCode(t *testing.T) {
	reg := rooms.NewRegistry()
	reg.Add(newRoom("WXYZ"))
	h, iss := newTestHandler(fakeVerifier{}, reg)

	rr := post(h.GuestLogin, "/auth/guest", `{"room":"room-1","code":"WXYZ"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	resp := decodeToken(t, rr)
	if resp.Room != "room-1" {
		t.Errorf("room = %q, want room-1", resp.Room)
	}
	claims, err := iss.Parse(resp.Token)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if claims.Role != token.RoleGuest || claims.Room != "room-1" {
		t.Errorf("claims = %+v, want guest bound to room-1", claims)
	}
}

func TestGuestLoginCaseInsensitiveCode(t *testing.T) {
	reg := rooms.NewRegistry()
	reg.Add(newRoom("WXYZ"))
	h, _ := newTestHandler(fakeVerifier{}, reg)

	rr := post(h.GuestLogin, "/auth/guest", `{"room":"room-1","code":"  wxyz  "}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (trimmed, lowercased code should match)", rr.Code)
	}
}

func TestGuestLoginWrongCode(t *testing.T) {
	reg := rooms.NewRegistry()
	reg.Add(newRoom("WXYZ"))
	h, _ := newTestHandler(fakeVerifier{}, reg)

	rr := post(h.GuestLogin, "/auth/guest", `{"room":"room-1","code":"ZZZZ"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestGuestLoginUnknownRoom(t *testing.T) {
	h, _ := newTestHandler(fakeVerifier{}, rooms.NewRegistry())
	rr := post(h.GuestLogin, "/auth/guest", `{"room":"nope","code":"WXYZ"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestGuestLoginRateLimited(t *testing.T) {
	reg := rooms.NewRegistry()
	reg.Add(newRoom("WXYZ"))
	h, _ := newTestHandler(fakeVerifier{}, reg)

	// Limiter allows 10 attempts/min; the 11th from the same IP is throttled.
	var last int
	for range 11 {
		rr := post(h.GuestLogin, "/auth/guest", `{"room":"room-1","code":"ZZZZ"}`)
		last = rr.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("11th attempt status = %d, want 429", last)
	}
}
