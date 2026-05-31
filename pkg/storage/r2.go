// Package storage provides a thin wrapper around Cloudflare R2 (S3-compatible).
// Used by media_service and file_service to store uploaded blobs in the cloud
// instead of local disk — so both developers share the same files.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2 is a Cloudflare R2 client. nil-safe: all methods are no-ops when r == nil.
type R2 struct {
	client    *s3.Client
	bucket    string
	publicURL string // e.g. https://pub-abc123.r2.dev  (no trailing slash)
}

// NewR2 creates an R2 client using S3-compatible credentials.
// endpoint — e.g. https://<account_id>.r2.cloudflarestorage.com
// publicURL — public bucket URL, e.g. https://pub-xxx.r2.dev
func NewR2(endpoint, accessKey, secretKey, bucket, publicURL string) (*R2, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" || publicURL == "" {
		return nil, fmt.Errorf("all R2 fields are required")
	}
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})
	return &R2{
		client:    client,
		bucket:    bucket,
		publicURL: strings.TrimRight(publicURL, "/"),
	}, nil
}

// Upload puts data under the given key and returns the public URL.
// key example: "uploads/2026/01/01/abc123.jpg"
func (r *R2) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("r2 not configured")
	}
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload %s: %w", key, err)
	}
	return r.publicURL + "/" + key, nil
}

// Delete removes the object at key. Silently ignores missing objects.
func (r *R2) Delete(ctx context.Context, key string) error {
	if r == nil {
		return nil
	}
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	return err
}

// URL returns the public URL for the given object key.
func (r *R2) URL(key string) string {
	if r == nil {
		return ""
	}
	return r.publicURL + "/" + key
}

// Download fetches the object at key and returns its bytes.
func (r *R2) Download(ctx context.Context, key string) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("r2 not configured")
	}
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("r2 download %s: %w", key, err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// KeyFromURL extracts the R2 object key from a full public URL.
// Returns ("", false) if the URL doesn't belong to this bucket.
func (r *R2) KeyFromURL(url string) (string, bool) {
	if r == nil {
		return "", false
	}
	prefix := r.publicURL + "/"
	if !strings.HasPrefix(url, prefix) {
		return "", false
	}
	return strings.TrimPrefix(url, prefix), true
}
