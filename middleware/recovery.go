package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"trpc.group/trpc-go/trpc-go/log"

	"timespace/util"
)

// RecoveryMiddlewareHTTP panic 恢复中间件，避免单个请求 panic 导致服务挂掉
func RecoveryMiddlewareHTTP(next HTTPHandler) HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				log.WithContext(r.Context()).Errorf("[PANIC] %s %s | err=%v\n%s",
					r.Method, r.URL.Path, rec, string(stack))
				util.Error(w, 500, "服务器内部错误")
				err = fmt.Errorf("panic recovered: %v", rec)
			}
		}()
		return next(w, r)
	}
}
