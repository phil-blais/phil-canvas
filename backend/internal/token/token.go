// Package token issues and verifies the Go-issued JWTs that authorize access to
// rooms and protected endpoints. Tokens are signed with HS256 using a symmetric
// secret.
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role is the authorization role carried by a token.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleGuest Role = "guest"
)

// Claims is the payload of a Go-issued JWT. Room is empty for admins, whose
// authorization is by room ownership rather than a bound room claim. Name is
// only ever set for admins — a display name (falling back to email, then UID)
// used to attribute who saved a version, not for authorization.
type Claims struct {
	Role Role   `json:"role"`
	Room string `json:"room,omitempty"`
	Name string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

// Issuer signs and verifies Claims.
type Issuer struct {
	secret []byte
	ttl    time.Duration
	// now is overridable in tests.
	now func() time.Time
}

// NewIssuer returns an Issuer signing tokens with the given secret and TTL.
func NewIssuer(secret []byte, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, ttl: ttl, now: time.Now}
}

// IssueAdmin returns a signed admin token for the given Firebase UID. Admin
// tokens carry no room claim. name is a display name for attribution (e.g.
// "saved by"); callers should already have resolved it to a sensible
// fallback (email, then UID) if the account has no display name.
func (i *Issuer) IssueAdmin(uid, name string) (string, error) {
	return i.issue(Claims{Role: RoleAdmin, Name: name}, uid)
}

// IssueGuest returns a signed guest token bound to a specific room.
func (i *Issuer) IssueGuest(uid, room string) (string, error) {
	return i.issue(Claims{Role: RoleGuest, Room: room}, uid)
}

func (i *Issuer) issue(claims Claims, uid string) (string, error) {
	now := i.now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Subject:   uid,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

// Parse verifies a token's signature and expiry and returns its claims.
func (i *Issuer) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if claims.Role != RoleAdmin && claims.Role != RoleGuest {
		return nil, fmt.Errorf("invalid role %q", claims.Role)
	}
	return claims, nil
}
