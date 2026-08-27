package scenes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const scenesCollection = "scenes"
const versionsCollection = "versions"

// FirestoreStore is a Firestore-backed Store.
type FirestoreStore struct {
	client *firestore.Client
}

// NewFirestoreStore builds a FirestoreStore from a shared Firebase app.
func NewFirestoreStore(ctx context.Context, app *firebase.App) (*FirestoreStore, error) {
	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return &FirestoreStore{client: client}, nil
}

// Close releases the Firestore client.
func (s *FirestoreStore) Close() error { return s.client.Close() }

func (s *FirestoreStore) CreateScene(ctx context.Context, name, savedBy string, v Version) (string, string, error) {
	sceneRef := s.client.Collection(scenesCollection).NewDoc()
	versionRef := sceneRef.Collection(versionsCollection).NewDoc()

	// Write the scene doc and its first version atomically.
	err := s.client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if err := tx.Set(sceneRef, map[string]any{
			"name":      name,
			"createdAt": firestore.ServerTimestamp,
			"createdBy": savedBy,
		}); err != nil {
			return err
		}
		return tx.Set(versionRef, versionDoc(savedBy, v))
	})
	if err != nil {
		return "", "", err
	}
	return sceneRef.ID, versionRef.ID, nil
}

func (s *FirestoreStore) AddVersion(ctx context.Context, sceneID, savedBy string, v Version) (string, error) {
	versionRef := s.client.Collection(scenesCollection).Doc(sceneID).Collection(versionsCollection).NewDoc()
	if _, err := versionRef.Set(ctx, versionDoc(savedBy, v)); err != nil {
		return "", err
	}
	return versionRef.ID, nil
}

func (s *FirestoreStore) ListScenes(ctx context.Context) ([]SceneSummary, error) {
	iter := s.client.Collection(scenesCollection).OrderBy("createdAt", firestore.Asc).Documents(ctx)
	defer iter.Stop()

	var out []SceneSummary
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var d struct {
			Name      string    `firestore:"name"`
			CreatedAt time.Time `firestore:"createdAt"`
			CreatedBy string    `firestore:"createdBy"`
		}
		if err := doc.DataTo(&d); err != nil {
			return nil, err
		}
		out = append(out, SceneSummary{ID: doc.Ref.ID, Name: d.Name, CreatedAt: d.CreatedAt, CreatedBy: d.CreatedBy})
	}
	return out, nil
}

func (s *FirestoreStore) ListVersions(ctx context.Context, sceneID string) ([]VersionSummary, error) {
	iter := s.client.Collection(scenesCollection).Doc(sceneID).Collection(versionsCollection).
		OrderBy("createdAt", firestore.Asc).Documents(ctx)
	defer iter.Stop()

	var out []VersionSummary
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var d struct {
			CreatedAt time.Time `firestore:"createdAt"`
			SavedBy   string    `firestore:"savedBy"`
		}
		if err := doc.DataTo(&d); err != nil {
			return nil, err
		}
		out = append(out, VersionSummary{ID: doc.Ref.ID, CreatedAt: d.CreatedAt, SavedBy: d.SavedBy})
	}
	return out, nil
}

func (s *FirestoreStore) RenameScene(ctx context.Context, sceneID, name string) error {
	_, err := s.client.Collection(scenesCollection).Doc(sceneID).
		Update(ctx, []firestore.Update{{Path: "name", Value: name}})
	if status.Code(err) == codes.NotFound {
		return ErrNotFound
	}
	return err
}

// DeleteScene removes the scene doc and its version subcollection. Firestore's
// Go client has no recursive-delete primitive, so versions are deleted with a
// BulkWriter before the scene doc itself. This is not atomic with the version
// deletes: a crash partway through can leave orphaned version docs with no
// parent scene doc. Acceptable for this app's scale; a 500-op transaction
// would be the wrong tool anyway, since version counts are unbounded.
func (s *FirestoreStore) DeleteScene(ctx context.Context, sceneID string) error {
	sceneRef := s.client.Collection(scenesCollection).Doc(sceneID)
	if _, err := sceneRef.Get(ctx); err != nil {
		if status.Code(err) == codes.NotFound {
			return ErrNotFound
		}
		return err
	}

	bw := s.client.BulkWriter(ctx)
	iter := sceneRef.Collection(versionsCollection).DocumentRefs(ctx)
	for {
		ref, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return err
		}
		if _, err := bw.Delete(ref); err != nil {
			return err
		}
	}
	bw.End()

	_, err := sceneRef.Delete(ctx)
	return err
}

func (s *FirestoreStore) GetVersion(ctx context.Context, sceneID, versionID string) (*Version, error) {
	doc, err := s.client.Collection(scenesCollection).Doc(sceneID).
		Collection(versionsCollection).Doc(versionID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var d struct {
		Elements string   `firestore:"elements"`
		AppState string   `firestore:"appState"`
		FileIDs  []string `firestore:"fileIds"`
	}
	if err := doc.DataTo(&d); err != nil {
		return nil, err
	}
	return &Version{
		Elements: json.RawMessage(d.Elements),
		AppState: json.RawMessage(d.AppState),
		FileIDs:  d.FileIDs,
	}, nil
}

// versionDoc builds the Firestore representation of a version. Elements and
// appState are stored as JSON strings.
func versionDoc(savedBy string, v Version) map[string]any {
	return map[string]any{
		"createdAt": firestore.ServerTimestamp,
		"savedBy":   savedBy,
		"elements":  string(v.Elements),
		"appState":  string(v.AppState),
		"fileIds":   v.FileIDs,
	}
}
