package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"

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
