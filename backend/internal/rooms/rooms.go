// Package rooms holds the in-memory registry of collaboration rooms. Rooms are
// ephemeral: they live only in service memory and are lost on restart, which is
// acceptable by design.
//
// The backend is a pure relay, so a Room holds no canvas/document state — only
// the metadata needed to authorize, list, and broadcast to it.
package rooms

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// CloseCodeRoomClosed is the WebSocket close code sent when an admin closes a
// room. Clients treat it as a signal to return to the main site.
const CloseCodeRoomClosed = 4001

// RoomClosedMessage is the text frame broadcast just before closing a room's
// connections. It is the redundant half of the defensive-close signal, guarding
// against a proxy normalizing the close code.
var RoomClosedMessage = []byte(`{"type":"room-closed"}`)

// MessageKind distinguishes text from binary frames without coupling this
// package to a specific transport. Yjs sync/awareness frames are binary; control
// messages like room-closed are text.
type MessageKind int

const (
	TextMessage MessageKind = iota + 1
	BinaryMessage
)

// Client is a connected participant. The WebSocket relay implements it; the
// rooms package uses it to count participants, broadcast, and close connections
// without depending on the transport.
type Client interface {
	// Send queues an opaque message of the given kind for delivery to this client.
	Send(kind MessageKind, msg []byte)
	// CloseWithCode closes the client's connection with a WebSocket close code.
	CloseWithCode(code int, reason string)
}

// Room is a single collaboration room.
type Room struct {
	ID        string
	Code      string // 4-char guest auth code; uppercase, ambiguous glyphs excluded
	AdminUID  string // Firebase UID of the creating admin
	CreatedAt time.Time

	mu      sync.Mutex
	name    string // document title; defaults to "Untitled", editable at any time
	sceneID string // Firestore scene ID; empty until first save, then fixed
	clients map[Client]struct{}
}

// NewRoom builds a fully-initialized Room. Registry.Create uses it for the
// production path; it is also handy for constructing rooms with known fields
// (e.g. in tests or future seeding).
func NewRoom(id, name, code, adminUID, sceneID string) *Room {
	return &Room{
		ID:        id,
		name:      name,
		Code:      code,
		AdminUID:  adminUID,
		CreatedAt: time.Now(),
		sceneID:   sceneID,
		clients:   make(map[Client]struct{}),
	}
}

// Name returns the room's current display title.
func (r *Room) Name() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name
}

// SetName renames the room. Callers are responsible for rejecting blank names
// where that matters (e.g. an explicit rename request); a blank name is only
// ever passed internally as "use the default", handled by Registry.Create.
func (r *Room) SetName(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.name = name
}

// SceneID returns the room's associated Firestore scene ID, or "" if none yet.
func (r *Room) SceneID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sceneID
}

// SetSceneID records the room's Firestore scene ID (set once, at first save).
func (r *Room) SetSceneID(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sceneID = id
}

// AddClient registers a connected client.
func (r *Room) AddClient(c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
}

// RemoveClient deregisters a client.
func (r *Room) RemoveClient(c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

// ParticipantCount returns the number of connected clients.
func (r *Room) ParticipantCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// Broadcast sends msg to every client except the sender, preserving frame kind.
func (r *Room) Broadcast(sender Client, kind MessageKind, msg []byte) {
	for _, c := range r.snapshotClients() {
		if c != sender {
			c.Send(kind, msg)
		}
	}
}

// CloseAll broadcasts message to all clients, then closes each connection with
// the room-closed code. It clears the client set so a client's own disconnect
// handler is idempotent.
func (r *Room) CloseAll(message []byte) {
	r.mu.Lock()
	clients := make([]Client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	r.clients = make(map[Client]struct{})
	r.mu.Unlock()

	for _, c := range clients {
		c.Send(TextMessage, message)
		c.CloseWithCode(CloseCodeRoomClosed, "room closed")
	}
}

// snapshotClients returns a copy of the current client set for lock-free iteration.
func (r *Room) snapshotClients() []Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	clients := make([]Client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	return clients
}

// Summary is the public view of a room (no code is ever exposed).
type Summary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ParticipantCount int    `json:"participantCount"`
	SceneID          string `json:"sceneId,omitempty"`
}

// Registry is a concurrency-safe in-memory set of rooms.
type Registry struct {
	mu        sync.RWMutex
	rooms     map[string]*Room
	bySceneID map[string]*Room // canonical live room for a scene, if any
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{rooms: make(map[string]*Room), bySceneID: make(map[string]*Room)}
}

// createLocked generates a room owned by adminUID and inserts it. Callers must
// hold r.mu for writing. name defaults to "Untitled" when blank.
func (r *Registry) createLocked(adminUID, name, sceneID string) *Room {
	if strings.TrimSpace(name) == "" {
		name = "Untitled"
	}
	var id string
	for {
		id = newRoomID()
		if _, exists := r.rooms[id]; !exists {
			break
		}
	}
	room := NewRoom(id, name, newCode(), adminUID, sceneID)
	r.rooms[id] = room
	return room
}

// Create generates a room owned by adminUID and inserts it. sceneID may be empty
// (blank room) or the ID of an existing scene the room is seeded from; in the
// latter case saves append to that scene. The seed JSON itself is not stored —
// the creating admin applies it client-side.
func (r *Registry) Create(adminUID, name, sceneID string) *Room {
	r.mu.Lock()
	defer r.mu.Unlock()
	room := r.createLocked(adminUID, name, sceneID)
	if sceneID != "" {
		r.bySceneID[sceneID] = room
	}
	return room
}

// GetOrCreateForScene returns the canonical live room for sceneID, creating one
// (seeded with name) if none is currently live. created is true iff a new room
// was made. The whole check-and-create happens under a single write lock, so
// two concurrent calls for the same sceneID can never both create a room.
func (r *Registry) GetOrCreateForScene(adminUID, sceneID, name string) (room *Room, created bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.bySceneID[sceneID]; ok {
		return existing, false
	}
	room = r.createLocked(adminUID, name, sceneID)
	r.bySceneID[sceneID] = room
	return room, true
}

// RoomForScene returns the currently-live room for sceneID, if any.
func (r *Registry) RoomForScene(sceneID string) (*Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.bySceneID[sceneID]
	return room, ok
}

// BindScene records that room is now the canonical live room for sceneID. Used
// when a previously-blank room (created with no sceneID) gets one attached by
// its first save — such a room was never indexed by scene at creation time.
func (r *Registry) BindScene(room *Room, sceneID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySceneID[sceneID] = room
}

// Add inserts a pre-built room into the registry (see NewRoom).
func (r *Registry) Add(room *Room) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[room.ID] = room
	if sceneID := room.SceneID(); sceneID != "" {
		r.bySceneID[sceneID] = room
	}
}

// Get returns the room with the given ID, if present.
func (r *Registry) Get(id string) (*Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.rooms[id]
	return room, ok
}

// Delete removes a room from the registry, including its scene index entry if
// it was the canonical live room for one.
func (r *Registry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	room, ok := r.rooms[id]
	if !ok {
		return
	}
	delete(r.rooms, id)
	if sceneID := room.SceneID(); sceneID != "" && r.bySceneID[sceneID] == room {
		delete(r.bySceneID, sceneID)
	}
}

// List returns a public summary of every open room, oldest first.
func (r *Registry) List() []Summary {
	r.mu.RLock()
	all := make([]*Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		all = append(all, room)
	}
	r.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })

	out := make([]Summary, 0, len(all))
	for _, room := range all {
		out = append(out, Summary{
			ID:               room.ID,
			Name:             room.Name(),
			ParticipantCount: room.ParticipantCount(),
			SceneID:          room.SceneID(),
		})
	}
	return out
}
