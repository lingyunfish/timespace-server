package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-go/log"

	"timespace/config"
)

// Storage 存储后端接口
type Storage interface {
	// Upload 上传文件，返回可访问的 URL
	Upload(ctx context.Context, reader io.Reader, size int64, contentType, ext string) (string, error)
}

var defaultStorage Storage

// Init 根据配置初始化存储后端
func Init() error {
	cfg := config.Get().Upload
	switch cfg.Driver {
	case "cos":
		s, err := NewCOSStorage(cfg.COS)
		if err != nil {
			return fmt.Errorf("init cos storage: %w", err)
		}
		defaultStorage = s
		log.Info("[INIT] storage: tencent cos")
	default:
		defaultStorage = NewLocalStorage(cfg.SavePath, cfg.URLPrefix)
		log.Info("[INIT] storage: local disk")
	}
	return nil
}

// Default 获取默认存储后端
func Default() Storage {
	return defaultStorage
}

// guessExtension 根据 contentType 推断扩展名
func guessExtension(contentType, originalName string) string {
	if ext := filepath.Ext(originalName); ext != "" {
		return ext
	}
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ".bin"
}

// genObjectKey 生成对象存储 key：uploads/2026/05/20/<uuid>.jpg
func genObjectKey(prefix, ext string) string {
	if prefix == "" {
		prefix = "uploads/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	now := time.Now()
	return fmt.Sprintf("%s%s/%s%s", prefix, now.Format("2006/01/02"), uuid.New().String(), ext)
}

// ============ 本地存储实现 ============

type LocalStorage struct {
	savePath  string // 物理保存目录，如 ./uploads
	urlPrefix string // URL 前缀，如 /static/uploads
}

func NewLocalStorage(savePath, urlPrefix string) *LocalStorage {
	return &LocalStorage{savePath: savePath, urlPrefix: urlPrefix}
}

func (s *LocalStorage) Upload(ctx context.Context, reader io.Reader, size int64, contentType, originalName string) (string, error) {
	ext := guessExtension(contentType, originalName)
	now := time.Now()
	dir := filepath.Join(s.savePath, now.Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := uuid.New().String() + ext
	fullPath := filepath.Join(dir, filename)

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, reader); err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/%s", s.urlPrefix, now.Format("2006/01/02"), filename)
	return url, nil
}
