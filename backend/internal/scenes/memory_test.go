package scenes

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func testVersion(elements string) Version {
	return Version{Elements: json.RawMessage(elements), AppState: json.RawMessage(`{}`)}
}

func TestMemStoreCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	sceneID, versionID, err := s.CreateScene(ctx, "My Scene", "uid-1", testVersion(`[{"id":"a"}]`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sceneID == "" || versionID == "" {
		t.Fatal("expected non-empty IDs")
	}

	got, err := s.GetVersion(ctx, sceneID, versionID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Elements) != `[{"id":"a"}]` {
		t.Errorf("elements = %s", got.Elements)
	}
}

func TestMemStoreAppendVersion(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	sceneID, v1, _ := s.CreateScene(ctx, "S", "uid", testVersion(`[1]`))
	v2, err := s.AddVersion(ctx, sceneID, "uid", testVersion(`[2]`))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if v1 == v2 {
		t.Error("expected distinct version IDs")
	}

	versions, err := s.ListVersions(ctx, sceneID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len = %d, want 2", len(versions))
	}
}

func TestMemStoreNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	if _, err := s.AddVersion(ctx, "nope", "uid", testVersion(`[]`)); err != ErrNotFound {
		t.Errorf("AddVersion err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetVersion(ctx, "nope", "nope"); err != ErrNotFound {
		t.Errorf("GetVersion err = %v, want ErrNotFound", err)
	}
	if _, err := s.ListVersions(ctx, "nope"); err != ErrNotFound {
		t.Errorf("ListVersions err = %v, want ErrNotFound", err)
	}
	if err := s.RenameScene(ctx, "nope", "New Name"); err != ErrNotFound {
		t.Errorf("RenameScene err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteScene(ctx, "nope"); err != ErrNotFound {
		t.Errorf("DeleteScene err = %v, want ErrNotFound", err)
	}
}

func TestMemStoreRenameScene(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	sceneID, _, _ := s.CreateScene(ctx, "Original", "uid", testVersion(`[]`))

	if err := s.RenameScene(ctx, sceneID, "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	list, err := s.ListScenes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Renamed" {
		t.Errorf("scenes = %+v, want a single scene named Renamed", list)
	}
}

func TestMemStoreDeleteScene(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	sceneID, _, _ := s.CreateScene(ctx, "Doomed", "uid", testVersion(`[]`))

	if err := s.DeleteScene(ctx, sceneID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetVersion(ctx, sceneID, "anything"); err != ErrNotFound {
		t.Errorf("GetVersion after delete err = %v, want ErrNotFound", err)
	}
	list, err := s.ListScenes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("scenes = %+v, want none after delete", list)
	}
}

func TestMemStoreListScenesOrdered(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	base := time.Unix(1000, 0)
	times := []time.Time{base.Add(time.Hour), base, base.Add(2 * time.Hour)}
	i := 0
	s.clock = func() time.Time { defer func() { i++ }(); return times[i] }

	s.CreateScene(ctx, "second", "u", testVersion(`[]`))
	s.CreateScene(ctx, "first", "u", testVersion(`[]`))
	s.CreateScene(ctx, "third", "u", testVersion(`[]`))

	list, err := s.ListScenes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"first", "second", "third"}
	for i, name := range want {
		if list[i].Name != name {
			t.Errorf("position %d = %q, want %q (oldest first)", i, list[i].Name, name)
		}
	}
}

func TestMemBlobStore(t *testing.T) {
	ctx := context.Background()
	b := NewMemBlobStore()

	exists, _ := b.Exists(ctx, "public/files/x")
	if exists {
		t.Fatal("should not exist yet")
	}
	if err := b.Put(ctx, "public/files/x", []byte("data"), "application/json"); err != nil {
		t.Fatalf("put: %v", err)
	}
	exists, _ = b.Exists(ctx, "public/files/x")
	if !exists {
		t.Fatal("should exist after put")
	}
	got, ok := b.Get("public/files/x")
	if !ok || string(got) != "data" {
		t.Errorf("get = %q, %v", got, ok)
	}
}
