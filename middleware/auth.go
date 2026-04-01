package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"timespace/config"
	"timespace/util"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// HTTPHandler tRPC-Go 泛HTTP标准服务的handler签名
type HTTPHandler func(http.ResponseWriter, *http.Request) error

func GetUserID(ctx context.Context) uint64 {
	if v, ok := ctx.Value(UserIDKey).(uint64); ok {
		return v
	}
	return 0
}

// GenerateToken 生成JWT token
func GenerateToken(userID uint64) (string, error) {
	cfg := config.Get().JWT
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Duration(cfg.ExpireHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// parseToken 解析JWT token
func parseToken(tokenStr string) (uint64, bool) {
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	cfg := config.Get().JWT
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return 0, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, false
	}
	return uint64(userIDFloat), true
}

// AuthMiddlewareHTTP 适配tRPC-Go泛HTTP标准服务的认证中间件
func AuthMiddlewareHTTP(next HTTPHandler) HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			util.Error(w, 401, "未登录")
			return nil
		}
		userID, ok := parseToken(tokenStr)
		if !ok {
			util.Error(w, 401, "登录已过期")
			return nil
		}
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		return next(w, r.WithContext(ctx))
	}
}

// OptionalAuthMiddlewareHTTP 可选认证（tRPC-Go HTTP版）
func OptionalAuthMiddlewareHTTP(next HTTPHandler) HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			return next(w, r)
		}
		userID, ok := parseToken(tokenStr)
		if !ok {
			return next(w, r)
		}
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		return next(w, r.WithContext(ctx))
	}
}

// CORSMiddlewareHTTP CORS中间件（tRPC-Go HTTP版）
func CORSMiddlewareHTTP(next HTTPHandler) HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return nil
		}
		return next(w, r)
	}
}

// AuthMiddleware JWT认证中间件 (标准http.HandlerFunc版)
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			util.Error(w, 401, "未登录")
			return
		}
		userID, ok := parseToken(tokenStr)
		if !ok {
			util.Error(w, 401, "登录已过期")
			return
		}
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// CORSMiddleware CORS中间件 (标准http.HandlerFunc版)
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}
