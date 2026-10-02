// Package s3store implements core.AssetStore backed by any S3-compatible
// object store (AWS S3, MinIO, Cloudflare R2, Backblaze B2, ...).
package s3store

import (
	"bytes"
	"context"
	"io"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oklog/ulid/v2"

	"github.com/Overview-Note/overview/internal/core"
)

// Store stores assets in an S3 bucket.
type Store struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

// Options configures an S3 asset store.
type Options struct {
	Endpoint  string // e.g. s3.amazonaws.com or minio.local:9000
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	PublicURL string // optional CDN/base URL prefix for returned asset URLs
}

// New creates an S3-backed asset store and ensures the bucket exists.
func New(ctx context.Context, opts Options) (*Store, error) {
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.UseSSL,
		Region: opts.Region,
	})
	if err != nil {
		return nil, err
	}
	exists, err := client.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, opts.Bucket, minio.MakeBucketOptions{Region: opts.Region}); err != nil {
			return nil, err
		}
	}
	return &Store{client: client, bucket: opts.Bucket, publicURL: strings.TrimRight(opts.PublicURL, "/")}, nil
}

// Save uploads r under a date-partitioned key derived from name.
func (s *Store) Save(ctx context.Context, name string, r io.Reader) (core.Asset, error) {
	ext := path.Ext(name)
	base := sanitize(strings.TrimSuffix(path.Base(name), ext))
	if base == "" {
		base = "asset"
	}
	rel := time.Now().UTC().Format("2006/01") + "/" + ulid.Make().String() + "-" + base + ext
	ct := mime.TypeByExtension(ext)

	// Buffer with a known size for a single PUT.
	data, err := io.ReadAll(r)
	if err != nil {
		return core.Asset{}, err
	}
	_, err = s.client.PutObject(ctx, s.bucket, rel, bytes.NewReader(data),
		int64(len(data)), minio.PutObjectOptions{ContentType: ct})
	if err != nil {
		return core.Asset{}, err
	}
	return core.Asset{
		Path:        rel,
		URL:         s.assetURL(rel),
		Name:        path.Base(name),
		Size:        int64(len(data)),
		ContentType: ct,
		Created:     time.Now().UTC(),
	}, nil
}

// Open returns a reader for the asset at rel.
func (s *Store) Open(ctx context.Context, rel string) (io.ReadSeekCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, rel, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return obj, nil
}

// ListAssets returns metadata for every stored asset.
func (s *Store) ListAssets(ctx context.Context) ([]core.Asset, error) {
	assets := make([]core.Asset, 0, 32)
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Recursive: true})
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		assets = append(assets, core.Asset{
			Path:        obj.Key,
			URL:         s.assetURL(obj.Key),
			Name:        path.Base(obj.Key),
			Size:        obj.Size,
			ContentType: contentType(obj.Key),
			Created:     obj.LastModified,
		})
	}
	return assets, nil
}

// DeleteAsset removes the asset at rel.
func (s *Store) DeleteAsset(ctx context.Context, rel string) error {
	return s.client.RemoveObject(ctx, s.bucket, rel, minio.RemoveObjectOptions{})
}

func (s *Store) assetURL(rel string) string {
	if s.publicURL != "" {
		return s.publicURL + "/" + rel
	}
	return "/assets/" + rel
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func contentType(p string) string {
	if ct := mime.TypeByExtension(path.Ext(p)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
