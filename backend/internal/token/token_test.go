package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAdminHasNoRoom(t *testing.T) {
	iss := NewIssuer([]byte("secret"), time.Hour)
	tok, err := iss.IssueAdmin("uid-1", "Ada Lovelace")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := iss.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Role != RoleAdmin {
		t.Errorf("role = %q, want admin", claims.Role)
	}
	if claims.Room != "" {
		t.Errorf("room = %q, want empty for admin", claims.Room)
	}
	if claims.Subject != "uid-1" {
		t.Errorf("subject = %q, want uid-1", claims.Subject)
	}
	if claims.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want Ada Lovelace", claims.Name)
	}
}

func TestIssueGuestBoundToRoom(t *testing.T) {
	iss := NewIssuer([]byte("secret"), time.Hour)
	tok, err := iss.IssueGuest("guest-1", "room-abc")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := iss.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Role != RoleGuest {
		t.Errorf("role = %q, want guest", claims.Role)
	}
	if claims.Room != "room-abc" {
		t.Errorf("room = %q, want room-abc", claims.Room)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	iss := NewIssuer([]byte("secret"), time.Hour)
	// Issue as if two hours ago so the one-hour token is already expired.
	iss.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	tok, err := iss.IssueAdmin("uid-1", "Admin One")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	iss.now = time.Now
	if _, err := iss.Parse(tok); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	signer := NewIssuer([]byte("secret-a"), time.Hour)
	tok, err := signer.IssueAdmin("uid-1", "Admin One")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verifier := NewIssuer([]byte("secret-b"), time.Hour)
	if _, err := verifier.Parse(tok); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}

func TestParseRejectsUnknownRole(t *testing.T) {
	secret := []byte("secret")
	// Craft a validly-signed token carrying an unknown role.
	claims := Claims{Role: "superuser", RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "uid-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewIssuer(secret, time.Hour).Parse(raw); err == nil {
		t.Fatal("expected unknown role to be rejected")
	}
}

func TestParseRejectsNoneAlgorithm(t *testing.T) {
	// A token using alg=none must never be accepted.
	claims := Claims{Role: RoleAdmin, RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "uid-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewIssuer([]byte("secret"), time.Hour).Parse(raw); err == nil {
		t.Fatal("expected alg=none token to be rejected")
	}
}
