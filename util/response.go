package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-go/log"
)

// APIResponse 统一API响应格式
type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := APIResponse{Code: code, Msg: msg, Data: data}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Errorf("write json response failed: %v", err)
	}
}

func Success(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, 0, "ok", data)
}

// Error 返回错误响应（不带详细错误，仅记录给前端的简短提示）
func Error(w http.ResponseWriter, code int, msg string) {
	WriteJSON(w, code, msg, nil)
}

// ErrorCtx 返回错误响应并记录详细错误日志（推荐在 handler 中使用）
func ErrorCtx(ctx context.Context, w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.WithContext(ctx).Errorf("[API ERR] code=%d msg=%s err=%v", code, msg, err)
	} else {
		log.WithContext(ctx).Warnf("[API ERR] code=%d msg=%s", code, msg)
	}
	WriteJSON(w, code, msg, nil)
}

// LogDBError 记录数据库错误日志
func LogDBError(ctx context.Context, op string, err error, args ...interface{}) {
	if err == nil {
		return
	}
	log.WithContext(ctx).Errorf("[DB ERR] op=%s err=%v args=%v", op, err, args)
}

// LogCacheError 记录缓存错误日志（缓存未命中等不算错误，调用方需自行判断）
func LogCacheError(ctx context.Context, op string, err error) {
	if err == nil {
		return
	}
	log.WithContext(ctx).Warnf("[CACHE ERR] op=%s err=%v", op, err)
}

func ParseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// BuildFullURL 把相对路径补全为完整 URL（用于图片等资源返回给前端）
// 如果传入已经是 http(s):// 开头的完整 URL，则原样返回
// 如果是 /static/... 这种相对路径，则拼上请求的 scheme + host
func BuildFullURL(r *http.Request, path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	scheme := "http"
	// 检测 https：1) TLS 直连 2) 反向代理 X-Forwarded-Proto
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return scheme + "://" + host + path
}

// CalcDistance 计算两点间距离（米），Haversine公式
func CalcDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// FormatDistance 格式化距离文本
func FormatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("%.0fm", meters)
	}
	return fmt.Sprintf("%.1fkm", meters/1000)
}

// FixURLs 递归遍历 data，把所有以 / 开头（如 /static/...）的字符串字段补全为完整 URL
// 用法：在 util.Success 之前调用 util.FixURLs(r, data)
// 作用对象：map[string]interface{}、[]interface{}、struct（通过 json marshal/unmarshal 转 map 处理）
func FixURLs(r *http.Request, data interface{}) interface{} {
	// 通过 JSON 互转把任意结构体转成可遍历的通用结构
	bs, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var v interface{}
	if err := json.Unmarshal(bs, &v); err != nil {
		return data
	}
	fixURLsRecursive(r, v)
	return v
}

func fixURLsRecursive(r *http.Request, v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if shouldFixURL(k, s) {
					t[k] = BuildFullURL(r, s)
				}
			} else {
				fixURLsRecursive(r, val)
			}
		}
	case []interface{}:
		for _, item := range t {
			fixURLsRecursive(r, item)
		}
	}
}

// shouldFixURL 判断某字段值是否应该被补全为完整 URL
func shouldFixURL(key, value string) bool {
	if value == "" {
		return false
	}
	// 已经是完整 URL 不处理
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return false
	}
	// 只处理以 / 开头的相对路径
	if !strings.HasPrefix(value, "/") {
		return false
	}
	// 字段名包含 url / image / avatar / thumbnail / cover / icon 等
	keyLower := strings.ToLower(key)
	for _, kw := range []string{"url", "image", "avatar", "thumbnail", "cover", "icon", "photo"} {
		if strings.Contains(keyLower, kw) {
			return true
		}
	}
	return false
}

// SuccessFixURL 包含图片等资源 URL 的成功响应（自动补全为完整 URL）
func SuccessFixURL(r *http.Request, w http.ResponseWriter, data interface{}) {
	Success(w, FixURLs(r, data))
}
