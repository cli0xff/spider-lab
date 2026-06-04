package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps MinIO operations for spider file storage.
type Client struct {
	mc     *minio.Client
	bucket string
}

// Config holds storage connection settings.
type Config struct {
	Endpoint  string // e.g. "storage:9000"
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// NewClient creates a MinIO storage client. Returns nil if endpoint is empty
// (storage disabled).
func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, nil
	}
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio connect: %w", err)
	}
	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

// Upload stores a file in MinIO. key is the object path, e.g.
// "spiders/{spider_id}/output/images/post_123_0.png".
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// UploadFile uploads a local file to MinIO.
func (c *Client) UploadFile(ctx context.Context, key, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	ct := detectContentType(localPath)
	return c.Upload(ctx, key, f, info.Size(), ct)
}

// Download returns a reader for an object in MinIO.
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", 0, err
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", 0, err
	}
	return obj, info.ContentType, info.Size, nil
}

// Exists checks if an object exists in MinIO.
func (c *Client) Exists(ctx context.Context, key string) bool {
	_, err := c.mc.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	return err == nil
}

// UploadDir uploads all files under a local directory to MinIO with the given
// key prefix. Returns the number of files uploaded.
func (c *Client) UploadDir(ctx context.Context, keyPrefix, localDir string) (int, error) {
	count := 0
	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(localDir, path)
		key := keyPrefix + "/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
		if uploadErr := c.UploadFile(ctx, key, path); uploadErr != nil {
			return uploadErr
		}
		count++
		return nil
	})
	return count, err
}

// HealthCheck pings the MinIO server.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.mc.BucketExists(ctx, c.bucket)
	return err
}

// ServeHTTP writes the object content directly to an http.ResponseWriter.
func (c *Client) ServeHTTP(ctx context.Context, w http.ResponseWriter, key string) error {
	reader, contentType, size, err := c.Download(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, err = io.Copy(w, reader)
	return err
}

func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
