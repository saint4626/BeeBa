package minio

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"beeba.org/internal/config"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client *miniogo.Client
}

func New(cfg config.Config) (*Client, error) {
	client, err := miniogo.New(cfg.MinIOEndpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	exists, err := c.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket %q: %w", bucket, err)
	}
	if exists {
		return nil
	}
	if err := c.client.MakeBucket(ctx, bucket, miniogo.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create minio bucket %q: %w", bucket, err)
	}
	return nil
}

func (c *Client) PutObject(ctx context.Context, bucket string, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, bucket, key, reader, size, miniogo.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put minio object %q/%q: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error) {
	object, err := c.client.GetObject(ctx, bucket, key, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get minio object %q/%q: %w", bucket, key, err)
	}
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, fmt.Errorf("stat minio object %q/%q: %w", bucket, key, err)
	}
	return object, nil
}

func (c *Client) GetObjectRange(ctx context.Context, bucket string, key string, start int64, end int64) (io.ReadCloser, error) {
	options := miniogo.GetObjectOptions{}
	if err := options.SetRange(start, end); err != nil {
		return nil, fmt.Errorf("set minio object range %q/%q %d-%d: %w", bucket, key, start, end, err)
	}
	object, err := c.client.GetObject(ctx, bucket, key, options)
	if err != nil {
		return nil, fmt.Errorf("get minio object range %q/%q %d-%d: %w", bucket, key, start, end, err)
	}
	return object, nil
}

func (c *Client) RemoveObject(ctx context.Context, bucket string, key string) error {
	if err := c.client.RemoveObject(ctx, bucket, key, miniogo.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove minio object %q/%q: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) CopyObject(ctx context.Context, sourceBucket string, sourceKey string, destinationBucket string, destinationKey string) error {
	_, err := c.client.CopyObject(ctx,
		miniogo.CopyDestOptions{Bucket: destinationBucket, Object: destinationKey},
		miniogo.CopySrcOptions{Bucket: sourceBucket, Object: sourceKey},
	)
	if err != nil {
		return fmt.Errorf("copy minio object %q/%q to %q/%q: %w", sourceBucket, sourceKey, destinationBucket, destinationKey, err)
	}
	return nil
}

func (c *Client) PresignedGetObject(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	link, err := c.client.PresignedGetObject(ctx, bucket, key, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("presign minio object %q/%q: %w", bucket, key, err)
	}
	return link.String(), nil
}
