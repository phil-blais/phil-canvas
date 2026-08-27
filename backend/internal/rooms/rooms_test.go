package rooms

import (
	"strings"
	"testing"
	"time"
)

type fakeClient struct {
	sent         [][]byte
	closed       bool
	closedCode   int
	closedReason string
}

func (f *fakeClient) Send(_ MessageKind, msg []byte) { f.sent = append(f.sent, msg) }
func (f *fakeClient) CloseWithCode(code int, reason string) {
	f.closed = true
	f.closedCode = code
	f.closedReason = reason
}

func TestCreateGeneratesFields(t *testing.T) {
	reg := NewRegistry()
	room := reg.Create("admin-uid", "", "")

	if len(room.ID) != 10 {
		t.Errorf("ID = %q, want 10 hex chars", room.ID)
	}
	if len(room.Code) != codeLength {
		t.Errorf("Code = %q, want %d chars", room.Code, codeLength)
	}
	for _, c := range room.Code {
		if !strings.ContainsRune(codeCharset, c) {
			t.Errorf("Code %q contains char %q outside charset", room.Code, c)
		}
	}
	if room.Name() != "Untitled" {
		t.Errorf("Name = %q, want Untitled default", room.Name())
	}
	if room.AdminUID != "admin-uid" {
		t.Errorf("AdminUID = %q, want admin-uid", room.AdminUID)
	}
	if room.SceneID() != "" {
		t.Errorf("SceneID = %q, want empty", room.SceneID())
	}
	if room.ParticipantCount() != 0 {
		t.Errorf("ParticipantCount = %d, want 0", room.ParticipantCount())
	}

	if reg.Create("admin-uid", "", "").ID == room.ID {
		t.Error("expected distinct IDs across Create calls")
	}
}

func TestCreateWithName(t *testing.T) {
	reg := NewRegistry()
	room := reg.Create("admin-uid", "My Diagram", "")
	if room.Name() != "My Diagram" {
		t.Errorf("Name = %q, want My Diagram", room.Name())
	}
	room.SetName("Renamed")
	if room.Name() != "Renamed" {
		t.Errorf("after SetName: %q, want Renamed", room.Name())
	}
}

func TestCreateWithSceneID(t *testing.T) {
	reg := NewRegistry()
	room := reg.Create("admin-uid", "", "scene-1")
	if room.SceneID() != "scene-1" {
		t.Errorf("SceneID = %q, want scene-1", room.SceneID())
	}
	room.SetSceneID("scene-2")
	if room.SceneID() != "scene-2" {
		t.Errorf("after SetSceneID: %q, want scene-2", room.SceneID())
	}
}

func TestParticipantsAndBroadcast(t *testing.T) {
	room := NewRoom("r", "n", "CODE", "admin", "")
	c1, c2 := &fakeClient{}, &fakeClient{}
	room.AddClient(c1)
	room.AddClient(c2)
	if room.ParticipantCount() != 2 {
		t.Fatalf("count = %d, want 2", room.ParticipantCount())
	}

	room.Broadcast(c1, BinaryMessage, []byte("hello"))
	if len(c1.sent) != 0 {
		t.Error("sender should not receive its own broadcast")
	}
	if len(c2.sent) != 1 || string(c2.sent[0]) != "hello" {
		t.Errorf("c2 received %v, want [hello]", c2.sent)
	}

	room.RemoveClient(c1)
	if room.ParticipantCount() != 1 {
		t.Errorf("count after remove = %d, want 1", room.ParticipantCount())
	}
}

func TestCloseAll(t *testing.T) {
	room := NewRoom("r", "n", "CODE", "admin", "")
	c1, c2 := &fakeClient{}, &fakeClient{}
	room.AddClient(c1)
	room.AddClient(c2)

	room.CloseAll(RoomClosedMessage)

	for i, c := range []*fakeClient{c1, c2} {
		if len(c.sent) != 1 || string(c.sent[0]) != string(RoomClosedMessage) {
			t.Errorf("client %d sent = %v, want the room-closed message", i, c.sent)
		}
		if !c.closed || c.closedCode != CloseCodeRoomClosed {
			t.Errorf("client %d closed=%v code=%d, want closed with %d", i, c.closed, c.closedCode, CloseCodeRoomClosed)
		}
	}
	if room.ParticipantCount() != 0 {
		t.Errorf("count after CloseAll = %d, want 0", room.ParticipantCount())
	}
}

func TestListOrderedOldestFirstNoCode(t *testing.T) {
	reg := NewRegistry()
	older := NewRoom("old", "alpha-river-11", "AAAA", "admin", "")
	older.CreatedAt = time.Now().Add(-time.Minute)
	newer := NewRoom("new", "beta-canyon-22", "BBBB", "admin", "")
	newer.CreatedAt = time.Now()
	reg.Add(newer)
	reg.Add(older)

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].ID != "old" || list[1].ID != "new" {
		t.Errorf("order = [%s %s], want [old new] (oldest first)", list[0].ID, list[1].ID)
	}
	// Summary carries no code — this is a compile-time guarantee, but assert the
	// listed name/count are the public fields we expect.
	if list[0].Name != "alpha-river-11" || list[0].ParticipantCount != 0 {
		t.Errorf("summary = %+v, unexpected", list[0])
	}
}

func TestListExposesSceneID(t *testing.T) {
	reg := NewRegistry()
	reg.Create("admin", "", "scene-1")

	list := reg.List()
	if len(list) != 1 || list[0].SceneID != "scene-1" {
		t.Errorf("summary = %+v, want sceneId = scene-1", list)
	}
}

func TestGetOrCreateForSceneCreatesOnce(t *testing.T) {
	reg := NewRegistry()

	room, created := reg.GetOrCreateForScene("admin", "scene-1", "My Doc")
	if !created {
		t.Fatal("expected the first call to create a room")
	}
	if room.Name() != "My Doc" || room.SceneID() != "scene-1" {
		t.Errorf("room = %+v, unexpected", room)
	}

	again, created := reg.GetOrCreateForScene("other-admin", "scene-1", "Ignored Name")
	if created {
		t.Error("expected the second call to join the existing room, not create one")
	}
	if again != room {
		t.Error("expected the same room instance back")
	}
	if again.Name() != "My Doc" {
		t.Errorf("existing room's name changed to %q, want unchanged", again.Name())
	}
}

func TestRoomForScene(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.RoomForScene("scene-1"); ok {
		t.Fatal("expected no room for an unknown scene")
	}

	room, _ := reg.GetOrCreateForScene("admin", "scene-1", "")
	found, ok := reg.RoomForScene("scene-1")
	if !ok || found != room {
		t.Errorf("RoomForScene = %v, %v, want the created room", found, ok)
	}
}

func TestBindSceneMakesBlankRoomDiscoverable(t *testing.T) {
	reg := NewRegistry()
	room := reg.Create("admin", "Untitled", "")

	if _, ok := reg.RoomForScene("scene-1"); ok {
		t.Fatal("a freshly-created blank room must not be indexed under any scene")
	}

	reg.BindScene(room, "scene-1")
	found, ok := reg.RoomForScene("scene-1")
	if !ok || found != room {
		t.Errorf("RoomForScene after BindScene = %v, %v, want the bound room", found, ok)
	}
}

func TestDeleteClearsSceneIndex(t *testing.T) {
	reg := NewRegistry()
	room, _ := reg.GetOrCreateForScene("admin", "scene-1", "My Doc")

	reg.Delete(room.ID)

	if _, ok := reg.RoomForScene("scene-1"); ok {
		t.Error("expected the scene index entry to be cleared on Delete")
	}
	// A subsequent GetOrCreateForScene for the same scene must create a fresh
	// room rather than resurrecting the deleted one.
	fresh, created := reg.GetOrCreateForScene("admin", "scene-1", "My Doc")
	if !created || fresh.ID == room.ID {
		t.Error("expected a new room after the old one was deleted")
	}
}
