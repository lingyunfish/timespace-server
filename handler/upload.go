package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"timespace/config"
	"timespace/middleware"
	"timespace/util"
)

// UploadFile 文件上传
func UploadFile(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	cfg := config.Get().Upload

	r.ParseMultipartForm(cfg.MaxSize)
	file, header, err := r.FormFile("file")
	if err != nil {
		util.Error(w, 400, "获取文件失败")
		return nil
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > cfg.MaxSize {
		util.Error(w, 400, "文件太大")
		return nil
	}

	// 检查文件类型
	contentType := header.Header.Get("Content-Type")
	allowed := false
	for _, t := range cfg.AllowedTypes {
		if strings.EqualFold(contentType, t) {
			allowed = true
			break
		}
	}
	if !allowed {
		util.Error(w, 400, "不支持的文件类型")
		return nil
	}

	// 生成文件路径
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".jpg"
		}
	}

	now := time.Now()
	dir := filepath.Join(cfg.SavePath, now.Format("2006/01/02"))
	os.MkdirAll(dir, 0755)

	filename := uuid.New().String() + ext
	savePath := filepath.Join(dir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.Error(w, 500, "保存文件失败")
		return nil
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.Error(w, 500, "保存文件失败")
		return nil
	}

	url := fmt.Sprintf("%s/%s/%s", cfg.URLPrefix, now.Format("2006/01/02"), filename)

	util.Success(w, map[string]interface{}{
		"url":      url,
		"filename": filename,
	})
	return nil
}
