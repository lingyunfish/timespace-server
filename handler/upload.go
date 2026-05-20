package handler

import (
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-go/log"

	"timespace/config"
	"timespace/middleware"
	"timespace/storage"
	"timespace/util"
)

// UploadFile 文件上传（统一通过 storage.Storage 接口，本地或 COS）
func UploadFile(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		util.ErrorCtx(ctx, w, 401, "未登录", nil)
		return nil
	}

	cfg := config.Get().Upload

	if err := r.ParseMultipartForm(cfg.MaxSize); err != nil {
		util.ErrorCtx(ctx, w, 400, "解析表单失败", err)
		return nil
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		util.ErrorCtx(ctx, w, 400, "获取文件失败", err)
		return nil
	}
	defer file.Close()

	if header.Size > cfg.MaxSize {
		util.ErrorCtx(ctx, w, 400, "文件太大", nil)
		return nil
	}

	contentType := header.Header.Get("Content-Type")
	allowed := false
	for _, t := range cfg.AllowedTypes {
		if strings.EqualFold(contentType, t) {
			allowed = true
			break
		}
	}
	if !allowed {
		util.ErrorCtx(ctx, w, 400, "不支持的文件类型", nil)
		return nil
	}

	store := storage.Default()
	if store == nil {
		util.ErrorCtx(ctx, w, 500, "存储未初始化", nil)
		return nil
	}

	url, err := store.Upload(ctx, file, header.Size, contentType, header.Filename)
	if err != nil {
		log.WithContext(ctx).Errorf("[UPLOAD] failed uid=%d size=%d err=%v", userID, header.Size, err)
		util.ErrorCtx(ctx, w, 500, "上传失败", err)
		return nil
	}

	log.WithContext(ctx).Infof("[UPLOAD] success uid=%d size=%d url=%s", userID, header.Size, url)
	util.SuccessFixURL(r, w, map[string]interface{}{
		"url": url,
	})
	return nil
}
