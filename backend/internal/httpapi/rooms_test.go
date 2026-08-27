package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

func newTestServer() (*rooms.Registry, *token.Issuer, http.Handler) {
	reg := rooms.NewRegistry()
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	h := NewRoomHandler(reg, auth.NewAuthenticator(iss))
	mux := http.NewServeMux()
	h.Routes(mux)
	return reg, iss, mux
}

func do(handler http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func adminToken(t *testing.T, iss *token.Issuer, uid string) string {
	t.Helper()
	tok, err := iss.IssueAdmin(uid, uid)
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	return tok
}

func guestToken(t *testing.T, iss *token.Issuer, uid, room string) string {
	t.Helper()
	tok, err := iss.IssueGuest(uid, room)
	if err != nil {
		t.Fatalf("issue guest token: %v", err)
	}
	return tok
}

func TestListIsPublicAndHidesCode(t *testing.T) {
	reg, _, srv := newTestServer()
	reg.Create("admin-uid", "", "")

	rr := do(srv, http.MethodGet, "/rooms", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if _, leaked := list[0]["code"]; leaked {
		t.Error("room list must not expose the guest code")
	}
	for _, key := range []string{"id", "name", "participantCount"} {
		if _, ok := list[0][key]; !ok {
			t.Errorf("summary missing %q", key)
		}
	}
}

func TestCreateRequiresAdmin(t *testing.T) {
	reg, iss, srv := newTestServer()

	// No token → 401 from middleware.
	if rr := do(srv, http.MethodPost, "/rooms", "{}", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", rr.Code)
	}

	// Guest token → 403.
	guest := guestToken(t, iss, "guest-1", "room-x")
	if rr := do(srv, http.MethodPost, "/rooms", "{}", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest: status = %d, want 403", rr.Code)
	}

	if len(reg.List()) != 0 {
		t.Error("no room should have been created by unauthorized requests")
	}
}

func TestCreateAsAdmin(t *testing.T) {
	reg, iss, srv := newTestServer()
	admin := adminToken(t, iss, "admin-uid")

	rr := do(srv, http.MethodPost, "/rooms", `{"sceneId":"scene-1"}`, admin)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID, Name, Code, SceneID string
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Code) != 4 {
		t.Errorf("code = %q, want 4 chars (admin sees code to share)", resp.Code)
	}
	if resp.SceneID != "scene-1" {
		t.Errorf("sceneId = %q, want scene-1", resp.SceneID)
	}
	room, ok := reg.Get(resp.ID)
	if !ok || room.AdminUID != "admin-uid" {
		t.Errorf("room not registered to creating admin")
	}
}

func TestCreateAcceptsEmptyBody(t *testing.T) {
	_, iss, srv := newTestServer()
	admin := adminToken(t, iss, "admin-uid")
	// Blank room: no body at all.
	if rr := do(srv, http.MethodPost, "/rooms", "", admin); rr.Code != http.StatusCreated {
		t.Fatalf("empty body: status = %d, want 201", rr.Code)
	}
}

type fakeClient struct {
	sent   [][]byte
	closed bool
	code   int
}

func (f *fakeClient) Send(_ rooms.MessageKind, msg []byte) { f.sent = append(f.sent, msg) }
func (f *fakeClient) CloseWithCode(code int, _ string)     { f.closed = true; f.code = code }

func TestDeleteByOwnerClosesAndRemoves(t *testing.T) {
	reg, iss, srv := newTestServer()
	room := reg.Create("owner", "", "")
	client := &fakeClient{}
	room.AddClient(client)

	admin := adminToken(t, iss, "owner")
	rr := do(srv, http.MethodDelete, "/rooms/"+room.ID, "", admin)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if !client.closed || client.code != rooms.CloseCodeRoomClosed {
		t.Errorf("client closed=%v code=%d, want closed with %d", client.closed, client.code, rooms.CloseCodeRoomClosed)
	}
	if len(client.sent) != 1 || string(client.sent[0]) != string(rooms.RoomClosedMessage) {
		t.Errorf("client should have received the room-closed message, got %v", client.sent)
	}
	if _, ok := reg.Get(room.ID); ok {
		t.Error("room should be removed from the registry")
	}
}

func TestDeleteByAnyAdminSucceeds(t *testing.T) {
	reg, iss, srv := newTestServer()
	room := reg.Create("owner", "", "")

	other := adminToken(t, iss, "someone-else")
	rr := do(srv, http.MethodDelete, "/rooms/"+room.ID, "", other)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (any admin may close a shared document)", rr.Code)
	}
	if _, ok := reg.Get(room.ID); ok {
		t.Error("room should be removed from the registry")
	}
}

func TestDeleteByGuestIsForbidden(t *testing.T) {
	reg, iss, srv := newTestServer()
	room := reg.Create("owner", "", "")

	guest := guestToken(t, iss, "guest-1", room.ID)
	rr := do(srv, http.MethodDelete, "/rooms/"+room.ID, "", guest)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (guests cannot close rooms)", rr.Code)
	}
}

func TestDeleteMissingRoom(t *testing.T) {
	_, iss, srv := newTestServer()
	admin := adminToken(t, iss, "owner")
	rr := do(srv, http.MethodDelete, "/rooms/does-not-exist", "", admin)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestRenameRoomByAnyAdmin(t *testing.T) {
	reg, iss, srv := newTestServer()
	room := reg.Create("owner", "Untitled", "")
	other := adminToken(t, iss, "someone-else")

	rr := do(srv, http.MethodPatch, "/rooms/"+room.ID+"/name", `{"name":"My Diagram"}`, other)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if room.Name() != "My Diagram" {
		t.Errorf("room.Name() = %q, want My Diagram", room.Name())
	}
}

func TestRenameRoomRejectsBlankName(t *testing.T) {
	reg, iss, srv := newTestServer()
	room := reg.Create("owner", "Untitled", "")
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPatch, "/rooms/"+room.ID+"/name", `{"name":"  "}`, admin)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a blank name", rr.Code)
	}
	if room.Name() != "Untitled" {
		t.Errorf("room.Name() = %q, want unchanged Untitled", room.Name())
	}
}

func TestRenameMissingRoom(t *testing.T) {
	_, iss, srv := newTestServer()
	admin := adminToken(t, iss, "owner")
	rr := do(srv, http.MethodPatch, "/rooms/does-not-exist/name", `{"name":"X"}`, admin)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
