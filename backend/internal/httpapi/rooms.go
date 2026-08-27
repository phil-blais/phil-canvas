// Package httpapi hosts the REST handlers for resource endpoints: rooms and
// scenes. Auth flows live in the auth package; this package depends on auth
// for JWT middleware and authorization helpers.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
	"github.com/phil-blais/phil-canvas/backend/internal/web"
)

// RoomHandler serves the room lifecycle endpoints.
type RoomHandler struct {
	rooms *rooms.Registry
	authn *auth.Authenticator
}

// NewRoomHandler builds a RoomHandler.
func NewRoomHandler(reg *rooms.Registry, authn *auth.Authenticator) *RoomHandler {
	return &RoomHandler{rooms: reg, authn: authn}
}

// Routes registers the room endpoints. GET /rooms is public; the rest
// require a valid JWT (role is enforced inside the handlers).
func (h *RoomHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /rooms", h.List)
	mux.Handle("POST /rooms", h.authn.Middleware(http.HandlerFunc(h.Create)))
	mux.Handle("DELETE /rooms/{id}", h.authn.Middleware(http.HandlerFunc(h.Delete)))
	mux.Handle("PATCH /rooms/{id}/name", h.authn.Middleware(http.HandlerFunc(h.Rename)))
}

// List returns the public room list. Codes are never exposed.
func (h *RoomHandler) List(w http.ResponseWriter, _ *http.Request) {
	web.WriteJSON(w, http.StatusOK, h.rooms.List())
}

type createRoomRequest struct {
	// Name is the new document's initial title; defaults to "Untitled" when
	// blank.
	Name string `json:"name"`
	// SceneID optionally seeds the room from an existing saved scene so later
	// saves append to it. The seed JSON is applied client-side, not here.
	//
	// Deprecated: superseded by POST /scenes/{id}/open, which additionally
	// guarantees a single canonical live room per scene. Kept for one deploy
	// cycle so a not-yet-updated frontend build keeps working; remove once
	// the frontend has migrated (see docs/SRS.md §5 API Reference).
	SceneID string `json:"sceneId"`
}

type createRoomResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	SceneID string `json:"sceneId,omitempty"`
}

// Create makes a new room owned by the calling admin. Only admins may create
// rooms; the response includes the guest code for the admin to share.
func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	var req createRoomRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}

	room := h.rooms.Create(claims.Subject, req.Name, req.SceneID)
	web.WriteJSON(w, http.StatusCreated, createRoomResponse{
		ID:      room.ID,
		Name:    room.Name(),
		Code:    room.Code,
		SceneID: room.SceneID(),
	})
}

// Delete closes a room: it runs the defensive close (broadcast room-closed, then
// close each connection with code 4001) and removes the room. Any admin may
// close a room — documents are shared among admins, not owned by their creator.
func (h *RoomHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	room, ok := h.rooms.Get(id)
	if !ok {
		web.WriteError(w, http.StatusNotFound, "room not found")
		return
	}

	room.CloseAll(rooms.RoomClosedMessage)
	h.rooms.Delete(id)
	w.WriteHeader(http.StatusNoContent)
}

type renameRoomRequest struct {
	Name string `json:"name"`
}

// Rename retitles a not-yet-saved room's draft title. Once a room is
// scene-bound, PATCH /scenes/{id} is the source of truth for its name instead
// (see SceneHandler.Rename) — this endpoint only updates the in-memory room,
// so it has no lasting effect on a saved document's title beyond the life of
// its current live session.
func (h *RoomHandler) Rename(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	var req renameRoomRequest
	if !web.DecodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		web.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	room, ok := h.rooms.Get(r.PathValue("id"))
	if !ok {
		web.WriteError(w, http.StatusNotFound, "room not found")
		return
	}
	room.SetName(name)
	web.WriteJSON(w, http.StatusOK, map[string]string{"id": room.ID, "name": name})
}

// requireAdmin returns the claims if the caller is an admin, else writes 403.
func (h *RoomHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (*token.Claims, bool) {
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok || claims.Role != token.RoleAdmin {
		web.WriteError(w, http.StatusForbidden, "admin role required")
		return nil, false
	}
	return claims, true
}
