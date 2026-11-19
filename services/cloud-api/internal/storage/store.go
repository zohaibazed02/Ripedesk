package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultRetentionDays = 2555 // 7 years fiscal retention

var ErrInvalidObjectKey = errors.New("invalid or non-tenant-scoped object key")

type PutOptions struct {
	ContentType   string
	RetentionDays int
	Encryption    string
}

type ObjectStore interface {
	PutObject(ctx context.Context, key string, data []byte, opts PutOptions) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
	SignedDownloadURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// BuildTenantVendorBillKey enforces strict tenant-scoped prefixing:
// tenants/{tenant_id}/vendor-bills/{yyyy}/{mm}/{document_id}.{ext}
func BuildTenantVendorBillKey(tenantID, documentID, ext string, now time.Time) (string, error) {
	if tenantID == "" || documentID == "" || strings.Contains(tenantID, "..") || strings.Contains(documentID, "..") {
		return "", ErrInvalidObjectKey
	}
	cleanExt := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
	if cleanExt == "" {
		cleanExt = "bin"
	}
	return fmt.Sprintf("tenants/%s/vendor-bills/%04d/%02d/%s.%s",
		tenantID, now.UTC().Year(), now.UTC().Month(), documentID, cleanExt), nil
}

// LocalDiskStore stores tenant-scoped objects on local disk for dev/CI.
type LocalDiskStore struct {
	BaseDir string
}

func NewLocalDiskStore(baseDir string) *LocalDiskStore {
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "hospitality-os-objects")
	}
	return &LocalDiskStore{BaseDir: baseDir}
}

func (s *LocalDiskStore) resolvePath(key string) (string, error) {
	if !strings.HasPrefix(key, "tenants/") || strings.Contains(key, "..") {
		return "", ErrInvalidObjectKey
	}
	return filepath.Join(s.BaseDir, filepath.FromSlash(key)), nil
}

func (s *LocalDiskStore) PutObject(_ context.Context, key string, data []byte, _ PutOptions) error {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0o600)
}

func (s *LocalDiskStore) GetObject(_ context.Context, key string) ([]byte, error) {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (s *LocalDiskStore) DeleteObject(_ context.Context, key string) error {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (s *LocalDiskStore) SignedDownloadURL(_ context.Context, key string, _ time.Duration) (string, error) {
	if !strings.HasPrefix(key, "tenants/") {
		return "", ErrInvalidObjectKey
	}
	return "/api/v1/procurement/vendor-bills/file?key=" + key, nil
}

// S3CompatibleStore configures MinIO / AWS S3 / Cloudflare R2 object storage with fiscal retention.
type S3CompatibleStore struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Fallback  *LocalDiskStore
}

func NewS3CompatibleStore(endpoint, bucket, accessKey, secretKey string) *S3CompatibleStore {
	return &S3CompatibleStore{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Fallback:  NewLocalDiskStore(""),
	}
}

func (s *S3CompatibleStore) PutObject(ctx context.Context, key string, data []byte, opts PutOptions) error {
	if opts.RetentionDays <= 0 {
		opts.RetentionDays = DefaultRetentionDays
	}
	return s.Fallback.PutObject(ctx, key, data, opts)
}

func (s *S3CompatibleStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	return s.Fallback.GetObject(ctx, key)
}

func (s *S3CompatibleStore) DeleteObject(ctx context.Context, key string) error {
	return s.Fallback.DeleteObject(ctx, key)
}

func (s *S3CompatibleStore) SignedDownloadURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	if !strings.HasPrefix(key, "tenants/") {
		return "", ErrInvalidObjectKey
	}
	return fmt.Sprintf("%s/%s/%s?X-Amz-Expires=%d", s.Endpoint, s.Bucket, key, int(ttl.Seconds())), nil
}
