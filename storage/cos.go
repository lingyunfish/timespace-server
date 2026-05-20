package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"

	"timespace/config"
)

// COSStorage 腾讯云对象存储
type COSStorage struct {
	client     *cos.Client
	cdnDomain  string // CDN 加速域名（可选）
	bucketURL  string // 原始桶 URL
	pathPrefix string
}

// NewCOSStorage 创建 COS 存储实例
func NewCOSStorage(cfg config.COSConfig) (*COSStorage, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.BucketURL == "" {
		return nil, fmt.Errorf("cos config incomplete: secret_id/secret_key/bucket_url are required")
	}

	u, err := url.Parse(cfg.BucketURL)
	if err != nil {
		return nil, fmt.Errorf("invalid bucket_url: %w", err)
	}
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	prefix := cfg.PathPrefix
	if prefix == "" {
		prefix = "uploads/"
	}
	return &COSStorage{
		client:     client,
		cdnDomain:  strings.TrimRight(cfg.CDNDomain, "/"),
		bucketURL:  strings.TrimRight(cfg.BucketURL, "/"),
		pathPrefix: prefix,
	}, nil
}

// Upload 上传文件到 COS
func (s *COSStorage) Upload(ctx context.Context, reader io.Reader, size int64, contentType, originalName string) (string, error) {
	ext := guessExtension(contentType, originalName)
	objectKey := genObjectKey(s.pathPrefix, ext)

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		},
	}

	if _, err := s.client.Object.Put(ctx, objectKey, reader, opt); err != nil {
		return "", fmt.Errorf("cos put failed: %w", err)
	}

	// 优先返回 CDN 域名
	if s.cdnDomain != "" {
		return s.cdnDomain + "/" + objectKey, nil
	}
	return s.bucketURL + "/" + objectKey, nil
}
