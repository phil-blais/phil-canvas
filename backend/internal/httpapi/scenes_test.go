package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/phil-blais/phil-canvas/backend/internal/auth"
	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/scenes"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

func newSceneServer(t *testing.T) (*rooms.Registry, *scenes.MemStore, *scenes.MemBlobStore, *token.Issuer, http.Handler) {
	t.Helper()
	reg := rooms.NewRegistry()
	store := scenes.NewMemStore()
	blobs := scenes.NewMemBlobStore()
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	mux := http.NewServeMux()
	NewSceneHandler(store, blobs, reg, auth.NewAuthenticator(iss)).Routes(mux)
	return reg, store, blobs, iss, mux
}

func decodeSave(t *testing.T, body []byte) saveResponse {
	t.Helper()
	var resp saveResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode save response: %v (body=%s)", err, body)
	}
	return resp
}

func TestSaveFirstUsesRoomName(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "My Scene", "")
	admin := adminToken(t, iss, "owner")

	// The room's title (set at creation, or via rename) is used to name the
	// scene on first save — no name is ever passed in the save body.
	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[{"id":"a"}],"appState":{"zoom":1}}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("first save: status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	resp := decodeSave(t, rr.Body.Bytes())
	if resp.SceneID == "" || resp.VersionID == "" {
		t.Fatalf("expected sceneId and versionId, got %+v", resp)
	}
	if room.SceneID() != resp.SceneID {
		t.Errorf("room.SceneID = %q, want %q", room.SceneID(), resp.SceneID)
	}
	if found, ok := reg.RoomForScene(resp.SceneID); !ok || found != room {
		t.Error("first save should register the room in the scene index (BindScene)")
	}
}

func TestSaveAppendsOnSecondCall(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "S", "")
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[1],"appState":{}}`, admin)
	first := decodeSave(t, rr.Body.Bytes())

	// Second save appends to the same scene.
	rr = do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[1,2],"appState":{}}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("append save: status = %d, want 200", rr.Code)
	}
	second := decodeSave(t, rr.Body.Bytes())
	if second.SceneID != first.SceneID {
		t.Errorf("sceneId changed: %q -> %q", first.SceneID, second.SceneID)
	}
	if second.VersionID == first.VersionID {
		t.Error("expected a new version ID on append")
	}
}

// Regression: Save used to attribute every version to the room's creator
// (room.AdminUID), even when a different admin actually performed the save —
// wrong once any admin, not just the creator, was allowed to save a shared
// room. It must attribute whoever is actually calling.
func TestSaveAttributesActingAdminNotRoomCreator(t *testing.T) {
	reg, store, _, iss, srv := newSceneServer(t)
	room := reg.Create("creator-uid", "Shared Doc", "")
	other, err := iss.IssueAdmin("other-uid", "Grace Hopper")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", `{"elements":[1],"appState":{}}`, other)
	if rr.Code != http.StatusOK {
		t.Fatalf("save by non-creator admin: status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	saved := decodeSave(t, rr.Body.Bytes())

	list, err := store.ListScenes(context.Background())
	if err != nil {
		t.Fatalf("list scenes: %v", err)
	}
	if len(list) != 1 || list[0].CreatedBy != "Grace Hopper" {
		t.Fatalf("scenes = %+v, want createdBy = Grace Hopper (the acting admin, not the room creator)", list)
	}

	versions, err := store.ListVersions(context.Background(), saved.SceneID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || versions[0].SavedBy != "Grace Hopper" {
		t.Fatalf("versions = %+v, want savedBy = Grace Hopper", versions)
	}
}

func TestSaveSeededRoomSkipsNamePrompt(t *testing.T) {
	reg, store, _, iss, srv := newSceneServer(t)
	// A room seeded from an existing scene has its SceneID already set to that
	// (pre-existing) scene, so the first save appends silently with no name.
	sceneID, _, err := store.CreateScene(context.Background(), "Seed", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("seed scene: %v", err)
	}
	room := reg.Create("owner", "Seed", sceneID)
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", `{"elements":[1],"appState":{}}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("seeded save: status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if got := decodeSave(t, rr.Body.Bytes()).SceneID; got != sceneID {
		t.Errorf("sceneId = %q, want %q", got, sceneID)
	}
}

func TestSaveRejectsEmptyElements(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", `{"appState":{}}`, admin)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing elements", rr.Code)
	}
}

func TestSaveAuthorization(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	body := `{"elements":[1],"appState":{}}`

	// Any admin may save a shared document, not just its creator.
	if rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", body, adminToken(t, iss, "other")); rr.Code != http.StatusOK {
		t.Errorf("other admin: status = %d, want 200", rr.Code)
	}
	// Guest.
	if rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", body, guestToken(t, iss, "guest-1", room.ID)); rr.Code != http.StatusForbidden {
		t.Errorf("guest: status = %d, want 403", rr.Code)
	}
	// Unknown room.
	if rr := do(srv, http.MethodPost, "/rooms/nope/save", body, adminToken(t, iss, "owner")); rr.Code != http.StatusNotFound {
		t.Errorf("unknown room: status = %d, want 404", rr.Code)
	}
	// No token.
	if rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save", body, ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rr.Code)
	}
}

func TestSaveStoresFilesContentAddressedAndDedups(t *testing.T) {
	reg, store, blobs, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	admin := adminToken(t, iss, "owner")

	// First save uploads file "abc".
	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[1],"appState":{},"files":{"abc":{"id":"abc","dataURL":"A"}}}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("save: status = %d (body=%s)", rr.Code, rr.Body.String())
	}
	stored, ok := blobs.Get("public/files/abc")
	if !ok {
		t.Fatal("file abc should be stored under public/files/")
	}
	first := decodeSave(t, rr.Body.Bytes())
	version, _ := store.GetVersion(context.Background(), first.SceneID, first.VersionID)
	if len(version.FileIDs) != 1 || version.FileIDs[0] != "abc" {
		t.Errorf("version fileIds = %v, want [abc]", version.FileIDs)
	}

	// Second save re-sends file "abc" with different bytes; content-addressed
	// dedup must NOT overwrite the existing blob.
	do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[1,2],"appState":{},"files":{"abc":{"id":"abc","dataURL":"DIFFERENT"}}}`, admin)
	after, _ := blobs.Get("public/files/abc")
	if string(after) != string(stored) {
		t.Errorf("dedup failed: blob was overwritten (%s -> %s)", stored, after)
	}
}

func TestSaveRejectsInvalidFileID(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	admin := adminToken(t, iss, "owner")

	// Path-traversal attempt in the file id.
	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[1],"appState":{},"files":{"../evil":{"id":"x"}}}`, admin)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid file id", rr.Code)
	}
}

func TestPublishWritesPublicSceneAndFiles(t *testing.T) {
	reg, _, blobs, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/publish",
		`{"elements":[{"id":"a"}],"appState":{"zoom":1},"files":{"f1":{"id":"f1","dataURL":"X"}}}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}

	published, ok := blobs.Get(publishedPath)
	if !ok {
		t.Fatal("published scene should be written to Storage")
	}
	var scene publishedScene
	if err := json.Unmarshal(published, &scene); err != nil {
		t.Fatalf("published scene not valid JSON: %v", err)
	}
	if string(scene.Elements) != `[{"id":"a"}]` {
		t.Errorf("published elements = %s", scene.Elements)
	}
	if len(scene.FileIDs) != 1 || scene.FileIDs[0] != "f1" {
		t.Errorf("published fileIds = %v, want [f1]", scene.FileIDs)
	}
	if _, ok := blobs.Get("public/files/f1"); !ok {
		t.Error("referenced file f1 should be uploaded on publish")
	}
}

func TestPublishByAnyAdmin(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	rr := do(srv, http.MethodPost, "/rooms/"+room.ID+"/publish",
		`{"elements":[1],"appState":{}}`, adminToken(t, iss, "someone-else"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (any admin may publish a shared document)", rr.Code)
	}
}

func TestSceneReadEndpoints(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "Canvas", "")
	admin := adminToken(t, iss, "owner")

	// Seed a scene through a save.
	saved := decodeSave(t, do(srv, http.MethodPost, "/rooms/"+room.ID+"/save",
		`{"elements":[{"id":"a"}],"appState":{"zoom":2},"files":{}}`, admin).Body.Bytes())

	// GET /scenes lists it.
	rr := do(srv, http.MethodGet, "/scenes", "", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("list scenes: status = %d", rr.Code)
	}
	var list []scenes.SceneSummary
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "Canvas" {
		t.Fatalf("scene list = %+v, want one 'Canvas'", list)
	}

	// GET /scenes/{id}/versions.
	rr = do(srv, http.MethodGet, "/scenes/"+saved.SceneID+"/versions", "", admin)
	var versions []scenes.VersionSummary
	json.Unmarshal(rr.Body.Bytes(), &versions)
	if len(versions) != 1 {
		t.Fatalf("versions = %d, want 1", len(versions))
	}

	// GET a specific version returns full data.
	rr = do(srv, http.MethodGet, "/scenes/"+saved.SceneID+"/versions/"+saved.VersionID, "", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("get version: status = %d", rr.Code)
	}
	var version scenes.Version
	json.Unmarshal(rr.Body.Bytes(), &version)
	if string(version.AppState) != `{"zoom":2}` {
		t.Errorf("version appState = %s, want {\"zoom\":2}", version.AppState)
	}

	// Unknown version → 404.
	if rr := do(srv, http.MethodGet, "/scenes/"+saved.SceneID+"/versions/nope", "", admin); rr.Code != http.StatusNotFound {
		t.Errorf("unknown version: status = %d, want 404", rr.Code)
	}
}

func TestSceneReadEndpointsRequireAdmin(t *testing.T) {
	reg, _, _, iss, srv := newSceneServer(t)
	room := reg.Create("owner", "", "")
	guest := guestToken(t, iss, "guest-1", room.ID)

	if rr := do(srv, http.MethodGet, "/scenes", "", guest); rr.Code != http.StatusForbidden {
		t.Errorf("guest list scenes: status = %d, want 403", rr.Code)
	}
}

func decodeOpen(t *testing.T, body []byte) openSceneResponse {
	t.Helper()
	var resp openSceneResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode open response: %v (body=%s)", err, body)
	}
	return resp
}

func TestOpenCreatesThenJoinsCanonicalRoom(t *testing.T) {
	_, store, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")
	sceneID, _, err := store.CreateScene(context.Background(), "Canvas", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("seed scene: %v", err)
	}

	first := decodeOpen(t, do(srv, http.MethodPost, "/scenes/"+sceneID+"/open", `{"name":"Canvas"}`, admin).Body.Bytes())
	if first.Live {
		t.Error("first open should create a new session, not join one")
	}
	if first.Name != "Canvas" || first.SceneID != sceneID || len(first.Code) != 4 {
		t.Errorf("first open response = %+v, unexpected", first)
	}

	// A second admin opening the same scene joins the same canonical room.
	second := decodeOpen(t, do(srv, http.MethodPost, "/scenes/"+sceneID+"/open", `{"name":"Canvas"}`,
		adminToken(t, iss, "someone-else")).Body.Bytes())
	if !second.Live {
		t.Error("second open should join the already-live session")
	}
	if second.ID != first.ID {
		t.Errorf("second open room id = %q, want the same room %q (no forking)", second.ID, first.ID)
	}
}

func TestOpenUnknownSceneStillSucceeds(t *testing.T) {
	// Matches today's POST /rooms {sceneId} behavior: no existence check
	// against Firestore, since seeding stays a client-side concern.
	_, _, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPost, "/scenes/does-not-exist/open", `{"name":"Untitled"}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestRenameSceneSyncsLiveRoomName(t *testing.T) {
	reg, store, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")
	sceneID, _, _ := store.CreateScene(context.Background(), "Original", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})
	room, _ := reg.GetOrCreateForScene("owner", sceneID, "Original")

	rr := do(srv, http.MethodPatch, "/scenes/"+sceneID, `{"name":"Renamed"}`, admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if room.Name() != "Renamed" {
		t.Errorf("live room name = %q, want Renamed", room.Name())
	}
	list, _ := store.ListScenes(context.Background())
	if len(list) != 1 || list[0].Name != "Renamed" {
		t.Errorf("stored scenes = %+v, want one Renamed", list)
	}
}

func TestRenameSceneRejectsBlankName(t *testing.T) {
	_, store, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")
	sceneID, _, _ := store.CreateScene(context.Background(), "Original", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})

	rr := do(srv, http.MethodPatch, "/scenes/"+sceneID, `{"name":"  "}`, admin)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestRenameUnknownScene(t *testing.T) {
	_, _, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodPatch, "/scenes/does-not-exist", `{"name":"X"}`, admin)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestDeleteSceneWhenOffline(t *testing.T) {
	_, store, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")
	sceneID, _, _ := store.CreateScene(context.Background(), "Doomed", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})

	rr := do(srv, http.MethodDelete, "/scenes/"+sceneID, "", admin)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", rr.Code, rr.Body.String())
	}
	list, _ := store.ListScenes(context.Background())
	if len(list) != 0 {
		t.Errorf("scenes = %+v, want none after delete", list)
	}
}

func TestDeleteSceneWhileLiveIsConflict(t *testing.T) {
	reg, store, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")
	sceneID, _, _ := store.CreateScene(context.Background(), "Live Doc", "owner",
		scenes.Version{Elements: json.RawMessage(`[]`), AppState: json.RawMessage(`{}`)})
	reg.GetOrCreateForScene("owner", sceneID, "Live Doc")

	rr := do(srv, http.MethodDelete, "/scenes/"+sceneID, "", admin)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 while a live session exists", rr.Code)
	}
	list, _ := store.ListScenes(context.Background())
	if len(list) != 1 {
		t.Error("scene must not be deleted while a live session exists")
	}
}

func TestDeleteUnknownScene(t *testing.T) {
	_, _, _, iss, srv := newSceneServer(t)
	admin := adminToken(t, iss, "owner")

	rr := do(srv, http.MethodDelete, "/scenes/does-not-exist", "", admin)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
