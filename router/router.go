package router

import (
	"net/http"
	"regexp"
	"strings"
)

// Route 一条路由
type Route struct {
	method  string
	pattern *regexp.Regexp
	handler func(http.ResponseWriter, *http.Request) error
	rawPath string
}

// Router 简易路由器，支持路径中 :param 占位符（例如 /api/places/:id/photos）
type Router struct {
	routes   []Route
	notFound func(http.ResponseWriter, *http.Request) error
}

// New 创建一个新的路由器
func New() *Router {
	return &Router{}
}

// Handle 注册路由
// 路径中可使用：
//   - :name 形式表示动态参数（不含/），例如 "/api/places/:id/photos"
//   - * 表示通配剩余路径（含/），例如 "/static/*"
func (r *Router) Handle(method, path string, h func(http.ResponseWriter, *http.Request) error) {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		switch {
		case strings.HasPrefix(p, ":"):
			parts[i] = `([^/]+)`
		case p == "*":
			parts[i] = `(.*)`
		default:
			parts[i] = regexp.QuoteMeta(p)
		}
	}
	pattern := "^" + strings.Join(parts, "/") + "$"
	r.routes = append(r.routes, Route{
		method:  method,
		pattern: regexp.MustCompile(pattern),
		handler: h,
		rawPath: path,
	})
}

// GET / POST / PUT / DELETE 便捷方法
func (r *Router) GET(path string, h func(http.ResponseWriter, *http.Request) error) {
	r.Handle(http.MethodGet, path, h)
}
func (r *Router) POST(path string, h func(http.ResponseWriter, *http.Request) error) {
	r.Handle(http.MethodPost, path, h)
}
func (r *Router) PUT(path string, h func(http.ResponseWriter, *http.Request) error) {
	r.Handle(http.MethodPut, path, h)
}
func (r *Router) DELETE(path string, h func(http.ResponseWriter, *http.Request) error) {
	r.Handle(http.MethodDelete, path, h)
}
func (r *Router) ANY(path string, h func(http.ResponseWriter, *http.Request) error) {
	r.Handle("", path, h)
}

// SetNotFound 设置 404 处理函数
func (r *Router) SetNotFound(h func(http.ResponseWriter, *http.Request) error) {
	r.notFound = h
}

// ServeHTTP 实现 http.Handler 接口，可直接传给 thttp.RegisterNoProtocolServiceMux
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range r.routes {
		if route.method != "" && route.method != req.Method {
			continue
		}
		if route.pattern.MatchString(req.URL.Path) {
			route.handler(w, req)
			return
		}
	}
	if r.notFound != nil {
		r.notFound(w, req)
		return
	}
	http.NotFound(w, req)
}

// PathParam 给定 URL 路径，按预设的路由模板提取路径变量
func PathParam(req *http.Request, template string) map[string]string {
	tplParts := strings.Split(template, "/")
	urlParts := strings.Split(req.URL.Path, "/")
	if len(tplParts) != len(urlParts) {
		return nil
	}
	params := make(map[string]string)
	for i, p := range tplParts {
		if strings.HasPrefix(p, ":") {
			params[p[1:]] = urlParts[i]
		}
	}
	return params
}
