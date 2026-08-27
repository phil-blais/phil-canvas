package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

func TestAuthorizeRoomGuest(t *testing.T) {
	reg := rooms.NewRegistry()

	guestToRoom1 := &token.Claims{Role: token.RoleGuest, Room: "room-1"}
	if err := AuthorizeRoom(reg, guestToRoom1, "room-1"); err != nil {
		t.Errorf("guest bound to room-1 should access room-1: %v", err)
	}
	if err := AuthorizeRoom(reg, guestToRoom1, "room-2"); err == nil {
		t.Error("guest bound to room-1 must not access room-2")
	}
}

func TestAuthorizeRoomAnyAdmin(t *testing.T) {
	reg := rooms.NewRegistry()
	reg.Add(rooms.NewRoom("room-1", "", "", "owner", ""))

	owner := &token.Claims{Role: token.RoleAdmin}
	owner.Subject = "owner"
	other := &token.Claims{Role: token.RoleAdmin}
	other.Subject = "someone-else"

	if err := AuthorizeRoom(reg, owner, "room-1"); err != nil {
		t.Errorf("creating admin should access the room: %v", err)
	}
	if err := AuthorizeRoom(reg, other, "room-1"); err != nil {
		t.Errorf("any other admin should also access the shared room: %v", err)
	}
	if err := AuthorizeRoom(reg, owner, "missing"); err == nil {
		t.Error("admin must not access a non-existent room")
	}
}

func TestMiddlewareRejectsMissingToken(t *testing.T) {
	auth := NewAuthenticator(token.NewIssuer([]byte("secret"), time.Hour))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	rr := httptest.NewRecorder()
	auth.Middleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestMiddlewarePassesValidTokenAndExposesClaims(t *testing.T) {
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	tok, err := iss.IssueAdmin("uid-1", "Admin One")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	var gotSubject string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			t.Error("claims missing from context")
		} else {
			gotSubject = claims.Subject
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	NewAuthenticator(iss).Middleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotSubject != "uid-1" {
		t.Errorf("subject in context = %q, want uid-1", gotSubject)
	}
}
