package scenes

import (
	"context"
	"errors"

	gcs "cloud.google.com/go/storage"
	firebase "firebase.google.com/go/v4"
)

// GCSBlobStore is a Firebase Storage (GCS) backed BlobStore.
type GCSBlobStore struct {
	bucket *gcs.BucketHandle
}

// NewGCSBlobStore builds a GCSBlobStore for the named bucket from a shared
// Firebase app.
func NewGCSBlobStore(ctx context.Context, app *firebase.App, bucketName string) (*GCSBlobStore, error) {
	client, err := app.Storage(ctx)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}
	return &GCSBlobStore{bucket: bucket}, nil
}

func (b *GCSBlobStore) Exists(ctx context.Context, path string) (bool, error) {
	_, err := b.bucket.Object(path).Attrs(ctx)
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *GCSBlobStore) Put(ctx context.Context, path string, data []byte, contentType string) error {
	w := b.bucket.Object(path).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}
