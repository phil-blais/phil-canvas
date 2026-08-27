package scenes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"
)

// MemStore is an in-memory Store. It backs unit tests and the dev fallback used
// when Firebase persistence is not configured. It is not durable.
type MemStore struct {
	mu     sync.Mutex
	scenes map[string]*memScene
	clock  func() time.Time
}

type memScene struct {
	summary  SceneSummary
	versions []memVersion
}

type memVersion struct {
	summary VersionSummary
	data    Version
}

// NewMemStore returns an empty in-memory Store.
func NewMemStore() *MemStore {
	return &MemStore{scenes: make(map[string]*memScene), clock: time.Now}
}

func (s *MemStore) CreateScene(_ context.Context, name, savedBy string, v Version) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	sceneID := randID()
	versionID := randID()
	s.scenes[sceneID] = &memScene{
		summary: SceneSummary{ID: sceneID, Name: name, CreatedAt: now, CreatedBy: savedBy},
		versions: []memVersion{{
			summary: VersionSummary{ID: versionID, CreatedAt: now, SavedBy: savedBy},
			data:    cloneVersion(v),
		}},
	}
	return sceneID, versionID, nil
}

func (s *MemStore) AddVersion(_ context.Context, sceneID, savedBy string, v Version) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scene, ok := s.scenes[sceneID]
	if !ok {
		return "", ErrNotFound
	}
	versionID := randID()
	scene.versions = append(scene.versions, memVersion{
		summary: VersionSummary{ID: versionID, CreatedAt: s.clock(), SavedBy: savedBy},
		data:    cloneVersion(v),
	})
	return versionID, nil
}

func (s *MemStore) ListScenes(context.Context) ([]SceneSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]SceneSummary, 0, len(s.scenes))
	for _, scene := range s.scenes {
		out = append(out, scene.summary)
	}
	slices.SortFunc(out, func(a, b SceneSummary) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

func (s *MemStore) ListVersions(_ context.Context, sceneID string) ([]VersionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scene, ok := s.scenes[sceneID]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]VersionSummary, 0, len(scene.versions))
	for _, v := range scene.versions {
		out = append(out, v.summary)
	}
	return out, nil
}

func (s *MemStore) RenameScene(_ context.Context, sceneID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scene, ok := s.scenes[sceneID]
	if !ok {
		return ErrNotFound
	}
	scene.summary.Name = name
	return nil
}

func (s *MemStore) DeleteScene(_ context.Context, sceneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.scenes[sceneID]; !ok {
		return ErrNotFound
	}
	delete(s.scenes, sceneID)
	return nil
}

func (s *MemStore) GetVersion(_ context.Context, sceneID, versionID string) (*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scene, ok := s.scenes[sceneID]
	if !ok {
		return nil, ErrNotFound
	}
	for _, v := range scene.versions {
		if v.summary.ID == versionID {
			data := cloneVersion(v.data)
			return &data, nil
		}
	}
	return nil, ErrNotFound
}

// MemBlobStore is an in-memory BlobStore.
type MemBlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

// NewMemBlobStore returns an empty in-memory BlobStore.
func NewMemBlobStore() *MemBlobStore {
	return &MemBlobStore{blobs: make(map[string][]byte)}
}

func (b *MemBlobStore) Exists(_ context.Context, path string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.blobs[path]
	return ok, nil
}

func (b *MemBlobStore) Put(_ context.Context, path string, data []byte, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blobs[path] = slices.Clone(data)
	return nil
}

// Get returns a stored blob, for test assertions.
func (b *MemBlobStore) Get(path string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.blobs[path]
	return data, ok
}

func cloneVersion(v Version) Version {
	return Version{
		Elements: slices.Clone(v.Elements),
		AppState: slices.Clone(v.AppState),
		FileIDs:  slices.Clone(v.FileIDs),
	}
}

func randID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("scenes: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
