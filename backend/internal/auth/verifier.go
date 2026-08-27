package auth

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// VerifiedUser is the subset of a verified Firebase identity we care about.
type VerifiedUser struct {
	UID   string
	Email string
	// Name is the account's display name, if Firebase has one on file. Empty
	// for accounts with no display name set (callers should fall back to
	// Email, then UID, for anything user-facing).
	Name string
}

// IDTokenVerifier verifies a Firebase ID token and returns the identity it
// asserts. Abstracted as an interface so handlers can be tested without a real
// Firebase project or emulator.
type IDTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*VerifiedUser, error)
}

// FirebaseVerifier verifies tokens via the Firebase Admin SDK.
type FirebaseVerifier struct {
	client *firebaseauth.Client
}

// NewFirebaseVerifier builds a verifier from a shared Firebase app. Against the
// Auth emulator the app is built without credentials; otherwise it uses the
// service account / Application Default Credentials.
func NewFirebaseVerifier(ctx context.Context, app *firebase.App) (*FirebaseVerifier, error) {
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("init firebase auth client: %w", err)
	}
	return &FirebaseVerifier{client: client}, nil
}

// Verify checks the ID token and returns the asserted identity.
func (v *FirebaseVerifier) Verify(ctx context.Context, idToken string) (*VerifiedUser, error) {
	tok, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	email, _ := tok.Claims["email"].(string)
	name, _ := tok.Claims["name"].(string)
	return &VerifiedUser{UID: tok.UID, Email: email, Name: name}, nil
}
