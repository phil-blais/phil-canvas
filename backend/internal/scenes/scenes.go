// Package scenes persists Excalidraw scene snapshots. A scene has an ordered
// history of versions; each version stores the elements array, the appState, and
// the IDs of any referenced image files. Image binaries live in a BlobStore
// (content-addressed), not in the version documents.
package scenes

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when a scene or version does not exist.
var ErrNotFound = errors.New("scene or version not found")

// Version is one saved snapshot of a canvas. Elements and AppState are stored
// opaquely as raw JSON.
type Version struct {
	Elements json.RawMessage `json:"elements"`
	AppState json.RawMessage `json:"appState"`
	FileIDs  []string        `json:"fileIds"`
}

// SceneSummary is a scene's metadata, for listing.
type SceneSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	// CreatedBy is a display name (falling back to email, then UID — see
	// auth.displayName) attributing who saved the first version, not
	// necessarily a UID.
	CreatedBy string `json:"createdBy"`
}

// VersionSummary is a version's metadata, for listing.
type VersionSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	// SavedBy is a display name (see SceneSummary.CreatedBy), not necessarily
	// a UID.
	SavedBy string `json:"savedBy"`
}

// Store persists scenes and their version history.
type Store interface {
	// CreateScene creates a named scene and writes its first version, returning
	// both generated IDs. savedBy attributes the save (see SceneSummary.CreatedBy).
	CreateScene(ctx context.Context, name, savedBy string, v Version) (sceneID, versionID string, err error)
	// AddVersion appends a version to an existing scene, returning its ID.
	AddVersion(ctx context.Context, sceneID, savedBy string, v Version) (versionID string, err error)
	// ListScenes returns all scenes, oldest first.
	ListScenes(ctx context.Context) ([]SceneSummary, error)
	// ListVersions returns a scene's versions, oldest first.
	ListVersions(ctx context.Context, sceneID string) ([]VersionSummary, error)
	// GetVersion returns a version's full data, or ErrNotFound.
	GetVersion(ctx context.Context, sceneID, versionID string) (*Version, error)
	// RenameScene updates a scene's display name, or ErrNotFound.
	RenameScene(ctx context.Context, sceneID, name string) error
	// DeleteScene removes a scene and its full version history, or ErrNotFound.
	DeleteScene(ctx context.Context, sceneID string) error
}

// BlobStore stores content-addressed blobs: image files at public/files/{id}
// and the published scene JSON.
type BlobStore interface {
	// Exists reports whether an object exists at path.
	Exists(ctx context.Context, path string) (bool, error)
	// Put writes data at path with the given content type.
	Put(ctx context.Context, path string, data []byte, contentType string) error
}
