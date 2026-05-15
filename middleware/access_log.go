package middleware

import (
	"net/http"
	"time"

	"trpc.group/trpc-go/trpc-go/log"
)

// statusRecorder 包装 ResponseWriter 以记录状态码
type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(b)
	sr.size += n
	return n, err
}

// AccessLogMiddlewareHTTP 记录每个请求的访问日志和耗时
func AccessLogMiddlewareHTTP(next HTTPHandler) HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		err := next(sr, r)

		duration := time.Since(start)
		clientIP := r.Header.Get("X-Real-IP")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Forwarded-For")
		}
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		userID := GetUserID(r.Context())
		query := r.URL.RawQuery
		if query != "" {
			query = "?" + query
		}

		// 错误请求和慢请求用 Warn/Error 级别
		logFn := log.Infof
		if err != nil || sr.status >= 500 {
			logFn = log.Errorf
		} else if sr.status >= 400 || duration > 2*time.Second {
			logFn = log.Warnf
		}

		logFn("[ACCESS] %s %s%s | status=%d size=%d dur=%v ip=%s uid=%d err=%v",
			r.Method, r.URL.Path, query, sr.status, sr.size, duration, clientIP, userID, err)

		return err
	}
}
