// Package relay implements the pure WebSocket broadcast relay. It parses no Yjs
// internals and holds no document state: after an auth handshake, every frame a
// client sends is rebroadcast opaquely to the room's other clients.
package relay

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

const (
	// closeCodeAuthFailed is sent when the auth handshake fails.
	closeCodeAuthFailed = 4003
	// authTimeout bounds how long we wait for the initial auth frame.
	authTimeout = 10 * time.Second
)

// authFrame is the first frame a client must send after connecting.
type authFrame struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Handler upgrades and services WebSocket connections at /ws/{id}.
type Handler struct {
	rooms    *rooms.Registry
	issuer   *token.Issuer
	upgrader websocket.Upgrader
}

// NewHandler builds a relay Handler.
func NewHandler(reg *rooms.Registry, issuer *token.Issuer) *Handler {
	return &Handler{
		rooms:  reg,
		issuer: issuer,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// In production the browser reaches this same-origin via Firebase
			// Hosting rewrites; in dev via the Vite proxy. Origin is not a
			// security boundary here (the JWT is), so allow the upgrade and let
			// the auth handshake gate access.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Routes registers the WebSocket endpoint.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/{id}", h.serve)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	room, ok := h.rooms.Get(roomID)
	if !ok {
		// Reject before upgrading so the client sees a plain 404.
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an error response.
	}

	if !h.authenticate(conn, roomID) {
		return
	}

	client := newClient(conn, room)
	room.AddClient(client)
	go client.writePump()
	client.readPump() // blocks until the connection ends
}

// authenticate reads and validates the initial auth frame. On success the read
// deadline is reset for normal operation and it returns true. On any failure it
// closes the connection with code 4003 and returns false.
func (h *Handler) authenticate(conn *websocket.Conn, roomID string) bool {
	_ = conn.SetReadDeadline(time.Now().Add(authTimeout))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		closeWith(conn, closeCodeAuthFailed, "auth frame required")
		return false
	}

	var frame authFrame
	if err := json.Unmarshal(msg, &frame); err != nil || frame.Type != "auth" {
		closeWith(conn, closeCodeAuthFailed, "malformed auth frame")
		return false
	}

	claims, err := h.issuer.Parse(frame.Token)
	if err != nil {
		closeWith(conn, closeCodeAuthFailed, "invalid token")
		return false
	}
	if err := auth.AuthorizeRoom(h.rooms, claims, roomID); err != nil {
		closeWith(conn, closeCodeAuthFailed, "not authorized for room")
		return false
	}

	// Clear the auth deadline; the pong handler manages read deadlines now.
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	return true
}

// closeWith writes a close frame with the given code and closes the connection.
func closeWith(conn *websocket.Conn, code int, reason string) {
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
	_ = conn.Close()
}
