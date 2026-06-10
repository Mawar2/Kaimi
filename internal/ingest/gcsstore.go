package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// shaMetadataKey is the custom object-metadata key under which GCSStore records
// our SHA-256. GCS exposes CRC32C/MD5 natively but not SHA-256, so we store the
// checksum the ingest stage computes as user metadata to power dedup.
const shaMetadataKey = "sha256"

// GCSStore is the production ObjectStore backed by Google Cloud Storage. Objects
// are written under bucket using the noticeID-prefixed names the ingest stage
// builds, e.g. "{noticeID}/raw/{filename}" and "{noticeID}/text/{filename}.txt".
type GCSStore struct {
	client *storage.Client
	bucket string
}

// NewGCSStore builds a GCSStore for bucket. The caller must call the returned
// closer when finished to release the client. bucket is the GCS bucket name (no
// gs:// prefix) — see GCS_SOLICITATIONS_BUCKET in the app config.
func NewGCSStore(ctx context.Context, bucket string, opts ...option.ClientOption) (*GCSStore, func() error, error) {
	if bucket == "" {
		return nil, nil, fmt.Errorf("gcsstore: bucket is required")
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("gcsstore: new client: %w", err)
	}
	return &GCSStore{client: client, bucket: bucket}, client.Close, nil
}

// Stat returns the SHA-256 previously recorded for object, or exists=false if the
// object is not present. A missing object is not an error.
func (s *GCSStore) Stat(ctx context.Context, object string) (sha256hex string, exists bool, err error) {
	attrs, err := s.client.Bucket(s.bucket).Object(object).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("gcsstore: stat %s: %w", object, err)
	}
	return attrs.Metadata[shaMetadataKey], true, nil
}

// Put writes data to object with the given content type, records sha256hex as
// object metadata, and returns the gs:// URI.
func (s *GCSStore) Put(ctx context.Context, object string, data []byte, contentType, sha256hex string) (string, error) {
	w := s.client.Bucket(s.bucket).Object(object).NewWriter(ctx)
	w.ContentType = contentType
	w.Metadata = map[string]string{shaMetadataKey: sha256hex}

	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("gcsstore: write %s: %w", object, err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gcsstore: close %s: %w", object, err)
	}
	return s.URI(object), nil
}

// Read returns the bytes stored under object.
func (s *GCSStore) Read(ctx context.Context, object string) ([]byte, error) {
	r, err := s.client.Bucket(s.bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcsstore: open %s: %w", object, err)
	}
	defer func() { _ = r.Close() }()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gcsstore: read %s: %w", object, err)
	}
	return data, nil
}

// URI returns the gs:// URI for object.
func (s *GCSStore) URI(object string) string {
	return fmt.Sprintf("gs://%s/%s", s.bucket, object)
}
