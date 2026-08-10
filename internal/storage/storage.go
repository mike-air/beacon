// Package storage signs short-lived URLs that let a client upload to and
// download from object storage WITHOUT the file bytes ever passing through the
// API process. The handler signs a URL; the browser talks straight to S3.
//
// Course mapping: Chapter 22 — file uploads with presigned S3 URLs. The chapter
// uses aws-sdk-go-v2's S3 presign client against S3 or a MinIO endpoint; we do
// the same, behind a small Storage interface so the rest of the app never sees
// the SDK. When no bucket is configured the constructor returns nil and the
// HTTP layer answers 501.
//
// NOTE (deviation): the course keeps the presign client construction inline in
// a handler; we wrap it in an interface (S3Storage) so it is swappable and so
// "unconfigured" is representable as a nil Storage the handlers check for.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ErrDisabled is returned by a disabled Storage; the HTTP layer maps it to 501.
var ErrDisabled = errors.New("storage is not configured")

// Storage signs upload (PUT) and download (GET) URLs for a single object key.
type Storage interface {
	// PresignUpload returns a URL the client PUTs the file bytes to.
	PresignUpload(ctx context.Context, key, contentType string) (string, error)
	// PresignDownload returns a URL the client GETs the file bytes from.
	PresignDownload(ctx context.Context, key string) (string, error)
}

// Config carries everything the S3 presigner needs. Mirrors the S3_* settings.
type Config struct {
	Bucket         string
	Region         string
	Endpoint       string // custom endpoint for MinIO/R2; empty = real AWS
	AccessKey      string
	SecretKey      string
	PresignTTL     time.Duration
	ForcePathStyle bool
}

// S3Storage presigns against an S3-compatible endpoint (AWS S3 or MinIO).
type S3Storage struct {
	presign *s3.PresignClient
	bucket  string
	ttl     time.Duration
}

// New builds an S3-backed Storage from cfg. A custom endpoint plus path-style
// addressing makes it work against MinIO in dev; leaving Endpoint empty uses
// real AWS S3. Returns (nil, nil) — disabled, not an error — when no bucket is
// set, so callers boot fine without storage.
func New(cfg Config) (Storage, error) {
	if cfg.Bucket == "" {
		return nil, nil
	}
	ttl := cfg.PresignTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	opts := s3.Options{
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: cfg.ForcePathStyle,
	}
	if cfg.Endpoint != "" {
		opts.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	client := s3.New(opts)
	return &S3Storage{
		presign: s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
		ttl:     ttl,
	}, nil
}

// PresignUpload signs a PUT URL for key, valid for the configured TTL.
func (s *S3Storage) PresignUpload(ctx context.Context, key, contentType string) (string, error) {
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	req, err := s.presign.PresignPutObject(ctx, in, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return "", fmt.Errorf("storage.PresignUpload: %w", err)
	}
	return req.URL, nil
}

// PresignDownload signs a GET URL for key, valid for the configured TTL.
func (s *S3Storage) PresignDownload(ctx context.Context, key string) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return "", fmt.Errorf("storage.PresignDownload: %w", err)
	}
	return req.URL, nil
}
