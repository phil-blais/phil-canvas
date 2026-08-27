package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/scenes"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
	"github.com/phil-blais/phil-canvas/backend/internal/web"
)

const (
	filesPrefix   = "public/files/"
	publishedPath = "public/published-scene.json"
	blobJSON      = "application/json"
)

// SceneHandler serves scene save/publish and read endpoints.
type SceneHandler struct {
	store scenes.Store
	blobs scenes.BlobStore
	rooms *rooms.Registry
	authn *auth.Authenticator
}

// NewSceneHandler builds a SceneHandler.
func NewSceneHandler(store scenes.Store, blobs scenes.BlobStore, reg *rooms.Registry, authn *auth.Authenticator) *SceneHandler {
	return &SceneHandler{store: store, blobs: blobs, rooms: reg, authn: authn}
}

// Routes registers the scene endpoints. All require a valid JWT and admin role.
func (h *SceneHandler) Routes(mux *http.ServeMux) {
	mux.Handle("POST /rooms/{id}/save", h.authn.Middleware(http.HandlerFunc(h.Save)))
	mux.Handle("POST /rooms/{id}/publish", h.authn.Middleware(http.HandlerFunc(h.Publish)))
	mux.Handle("GET /scenes", h.authn.Middleware(http.HandlerFunc(h.ListScenes)))
	mux.Handle("POST /scenes/{id}/open", h.authn.Middleware(http.HandlerFunc(h.Open)))
	mux.Handle("PATCH /scenes/{id}", h.authn.Middleware(http.HandlerFunc(h.Rename)))
	mux.Handle("DELETE /scenes/{id}", h.authn.Middleware(http.HandlerFunc(h.Delete)))
	mux.Handle("GET /scenes/{id}/versions", h.authn.Middleware(http.HandlerFunc(h.ListVersions)))
	mux.Handle("GET /scenes/{id}/versions/{vid}", h.authn.Middleware(http.HandlerFunc(h.GetVersion)))
}

type saveRequest struct {
	Elements json.RawMessage            `json:"elements"`
	AppState json.RawMessage            `json:"appState"`
	Files    map[string]json.RawMessage `json:"files"`
}

type saveResponse struct {
	SceneID   string `json:"sceneId"`
	VersionID string `json:"versionId"`
}

// Save writes the current canvas as a new version. The first save of a blank
// room creates the scene using the room's current title (see Room.Name); later
// saves append silently. A room's title always has a value (defaults to
// "Untitled"), so a name is never prompted for here.
func (h *SceneHandler) Save(w http.ResponseWriter, r *http.Request) {
	room, claims, ok := h.roomFromPath(w, r)
	if !ok {
		return
	}

	var req saveRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Elements) == 0 {
		web.WriteError(w, http.StatusBadRequest, "elements are required")
		return
	}

	fileIDs, ok := h.uploadFiles(w, r.Context(), req.Files)
	if !ok {
		return
	}
	version := scenes.Version{Elements: req.Elements, AppState: req.AppState, FileIDs: fileIDs}

	// Attribute to whoever is actually saving right now, not the room's
	// original creator — any admin may save a shared document, so those can
	// differ. Claims.Name is a display name (see token.Claims); Subject (the
	// UID) is only a fallback for the rare case a token predates it.
	savedBy := claims.Name
	if savedBy == "" {
		savedBy = claims.Subject
	}

	sceneID := room.SceneID()
	var versionID string
	var err error
	if sceneID == "" {
		sceneID, versionID, err = h.store.CreateScene(r.Context(), room.Name(), savedBy, version)
		if err == nil {
			room.SetSceneID(sceneID)
			h.rooms.BindScene(room, sceneID)
		}
	} else {
		versionID, err = h.store.AddVersion(r.Context(), sceneID, savedBy, version)
	}
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not save scene")
		return
	}
	web.WriteJSON(w, http.StatusOK, saveResponse{SceneID: sceneID, VersionID: versionID})
}

type publishRequest struct {
	Elements json.RawMessage            `json:"elements"`
	AppState json.RawMessage            `json:"appState"`
	Files    map[string]json.RawMessage `json:"files"`
}

type publishedScene struct {
	Elements json.RawMessage `json:"elements"`
	AppState json.RawMessage `json:"appState"`
	FileIDs  []string        `json:"fileIds"`
}

// Publish writes the current canvas to Storage as the public scene, uploading
// any referenced image files.
func (h *SceneHandler) Publish(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.roomFromPath(w, r); !ok {
		return
	}

	var req publishRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Elements) == 0 {
		web.WriteError(w, http.StatusBadRequest, "elements are required")
		return
	}

	fileIDs, ok := h.uploadFiles(w, r.Context(), req.Files)
	if !ok {
		return
	}

	data, err := json.Marshal(publishedScene{Elements: req.Elements, AppState: req.AppState, FileIDs: fileIDs})
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not encode scene")
		return
	}
	if err := h.blobs.Put(r.Context(), publishedPath, data, blobJSON); err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not publish scene")
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]bool{"published": true})
}

// ListScenes returns all saved scenes (admin only).
func (h *SceneHandler) ListScenes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	list, err := h.store.ListScenes(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not list scenes")
		return
	}
	web.WriteJSON(w, http.StatusOK, list)
}

// ListVersions returns a scene's version history (admin only).
func (h *SceneHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	list, err := h.store.ListVersions(r.Context(), r.PathValue("id"))
	if errors.Is(err, scenes.ErrNotFound) {
		web.WriteError(w, http.StatusNotFound, "scene not found")
		return
	}
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not list versions")
		return
	}
	web.WriteJSON(w, http.StatusOK, list)
}

// GetVersion returns a version's full data (admin only).
func (h *SceneHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	version, err := h.store.GetVersion(r.Context(), r.PathValue("id"), r.PathValue("vid"))
	if errors.Is(err, scenes.ErrNotFound) {
		web.WriteError(w, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not fetch version")
		return
	}
	web.WriteJSON(w, http.StatusOK, version)
}

// uploadFiles stores any new image files at public/files/{fileId} (content-
// addressed, so uploads are idempotent) and returns the sorted list of IDs. It
// writes an error response and returns ok=false on failure.
func (h *SceneHandler) uploadFiles(w http.ResponseWriter, ctx context.Context, files map[string]json.RawMessage) ([]string, bool) {
	ids := make([]string, 0, len(files))
	for id := range files {
		if !validFileID(id) {
			web.WriteError(w, http.StatusBadRequest, "invalid file id")
			return nil, false
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	for _, id := range ids {
		path := filesPrefix + id
		exists, err := h.blobs.Exists(ctx, path)
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, "could not check file")
			return nil, false
		}
		if exists {
			continue
		}
		if err := h.blobs.Put(ctx, path, files[id], blobJSON); err != nil {
			web.WriteError(w, http.StatusInternalServerError, "could not store file")
			return nil, false
		}
	}
	return ids, true
}

// roomFromPath loads the room named by the {id} path value, requiring the
// caller to be an admin, and returns their claims alongside it (e.g. for
// attributing a save). Any admin may act on any room — documents are shared,
// not owned by their creator. On failure it writes the response and returns
// ok=false.
func (h *SceneHandler) roomFromPath(w http.ResponseWriter, r *http.Request) (*rooms.Room, *token.Claims, bool) {
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return nil, nil, false
	}
	room, ok := h.rooms.Get(r.PathValue("id"))
	if !ok {
		web.WriteError(w, http.StatusNotFound, "room not found")
		return nil, nil, false
	}
	return room, claims, true
}

type openSceneRequest struct {
	// Name is the scene's current display title, as already known to the
	// caller from the GET /scenes list it's rendering — used to seed a
	// freshly-created room's title without an extra Firestore read.
	Name string `json:"name"`
	// VersionID optionally selects which saved version to seed a *newly
	// created* room from (rather than the latest). Ignored when the scene is
	// already live — joining the existing session always wins, since
	// restoring a past version into a live session is a distinct, explicit
	// action performed after joining (see VersionHistory on the frontend).
	VersionID string `json:"versionId"`
}

type openSceneResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	SceneID string `json:"sceneId"`
	// Live is true when this call joined an already-live session rather than
	// creating a new one. The seed JSON is never returned here either way —
	// same as room creation, seeding stays a client-side concern.
	Live bool `json:"live"`
}

// Open returns the single canonical live room for a scene, creating one (seeded
// from VersionID or, if unset, the latest version) if none is currently live.
// This does not validate that the scene exists in Firestore — consistent with
// today's POST /rooms {sceneId}, which never has either.
func (h *SceneHandler) Open(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	var req openSceneRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}

	sceneID := r.PathValue("id")
	room, created := h.rooms.GetOrCreateForScene(claims.Subject, sceneID, req.Name)
	web.WriteJSON(w, http.StatusOK, openSceneResponse{
		ID:      room.ID,
		Name:    room.Name(),
		Code:    room.Code,
		SceneID: sceneID,
		Live:    !created,
	})
}

type renameSceneRequest struct {
	Name string `json:"name"`
}

// Rename retitles a saved scene. If a room is currently live for it, the
// live session's title is updated too, so anyone in it sees the new name
// immediately rather than waiting for a poll.
func (h *SceneHandler) Rename(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	var req renameSceneRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		web.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	sceneID := r.PathValue("id")
	if err := h.store.RenameScene(r.Context(), sceneID, name); errors.Is(err, scenes.ErrNotFound) {
		web.WriteError(w, http.StatusNotFound, "scene not found")
		return
	} else if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not rename scene")
		return
	}

	if room, ok := h.rooms.RoomForScene(sceneID); ok {
		room.SetName(name)
	}
	web.WriteJSON(w, http.StatusOK, map[string]string{"id": sceneID, "name": name})
}

// Delete removes a saved scene and its version history. Refuses while a room
// is currently live for it (409) rather than force-closing the session —
// the admin must close it first.
func (h *SceneHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	sceneID := r.PathValue("id")
	if _, live := h.rooms.RoomForScene(sceneID); live {
		web.WriteError(w, http.StatusConflict, "close the live session before deleting this document")
		return
	}

	if err := h.store.DeleteScene(r.Context(), sceneID); errors.Is(err, scenes.ErrNotFound) {
		web.WriteError(w, http.StatusNotFound, "scene not found")
		return
	} else if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "could not delete scene")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// requireAdmin returns the claims if the caller is an admin, else writes 403.
func (h *SceneHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (*token.Claims, bool) {
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok || claims.Role != token.RoleAdmin {
		web.WriteError(w, http.StatusForbidden, "admin role required")
		return nil, false
	}
	return claims, true
}

// validFileID guards against path traversal: Excalidraw file IDs are content
// hashes, so restrict to a safe alphabet.
func validFileID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
