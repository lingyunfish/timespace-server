package main

import (
	"net/http"
	"strings"

	trpc "trpc.group/trpc-go/trpc-go"
	thttp "trpc.group/trpc-go/trpc-go/http"
	"trpc.group/trpc-go/trpc-go/log"

	"timespace/config"
	"timespace/db"
	"timespace/handler"
	"timespace/middleware"
)

func main() {
	// 加载业务配置
	cfg, err := config.Load("config/config.json")
	if err != nil {
		panic("load config failed: " + err.Error())
	}

	// 初始化MySQL
	if err := db.InitMySQL(); err != nil {
		log.Warnf("init mysql failed: %v, running without database", err)
	} else {
		defer db.CloseMySQL()
		log.Info("mysql connected")
	}

	// 初始化Redis
	if err := db.InitRedis(); err != nil {
		log.Warnf("init redis failed: %v, running without cache", err)
	} else {
		defer db.CloseRedis()
		log.Info("redis connected")
	}

	_ = cfg

	// 创建tRPC服务
	s := trpc.NewServer()

	// ============ 注册路由 ============

	// --- 用户相关 ---
	thttp.HandleFunc("/api/user/login", wrapHandler(handler.UserLogin))
	thttp.HandleFunc("/api/user/info", wrapAuth(handler.GetUserInfo))
	thttp.HandleFunc("/api/user/update", wrapAuth(handler.UpdateUserInfo))
	thttp.HandleFunc("/api/user/stats", wrapAuth(handler.GetUserStats))
	thttp.HandleFunc("/api/user/achievements", wrapAuth(handler.GetUserAchievements))
	thttp.HandleFunc("/api/user/photos", wrapAuth(handler.GetUserPhotos))
	thttp.HandleFunc("/api/user/favorites", wrapAuth(handler.GetUserFavorites))
	thttp.HandleFunc("/api/user/footprints", wrapAuth(handler.GetUserFootprints))

	// --- 记忆点相关 ---
	thttp.HandleFunc("/api/places/nearby", wrapOptionalAuth(handler.GetNearbyPlaces))
	thttp.HandleFunc("/api/places/search", wrapOptionalAuth(handler.SearchPlaces))
	thttp.HandleFunc("/api/places/create", wrapAuth(handler.CreatePlace))

	// --- 动态路由 (使用前缀匹配) ---
	// /api/places/{id}
	// /api/places/{id}/photos
	thttp.HandleFunc("/api/places/", handlePlaceRoutes)

	// --- 照片相关 ---
	thttp.HandleFunc("/api/photos/publish", wrapAuth(handler.PublishPhotos))
	// /api/photos/{id}
	// /api/photos/{id}/like
	// /api/photos/{id}/comments
	thttp.HandleFunc("/api/photos/", handlePhotoRoutes)

	// --- 文件上传 ---
	thttp.HandleFunc("/api/upload", wrapAuth(handler.UploadFile))

	// --- 收藏 ---
	thttp.HandleFunc("/api/favorite", wrapAuth(handler.FavoritePhoto))

	// --- 品牌记忆 ---
	thttp.HandleFunc("/api/brand/memories", wrapOptionalAuth(handler.GetBrandMemories))

	// --- 静态文件服务 ---
	thttp.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) error {
		http.StripPrefix("/static/", http.FileServer(http.Dir("."))).ServeHTTP(w, r)
		return nil
	})

	// 注册服务
	thttp.RegisterNoProtocolService(s.Service("trpc.timespace.capsule.http"))

	log.Info("timespace server starting...")
	if err := s.Serve(); err != nil {
		panic(err)
	}
}

// handlePlaceRoutes 处理地点相关动态路由
func handlePlaceRoutes(w http.ResponseWriter, r *http.Request) error {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/places/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		return nil
	}

	// /api/places/{id}/photos
	if len(parts) >= 2 && parts[1] == "photos" {
		return middleware.OptionalAuthMiddlewareHTTP(handler.GetPlacePhotos)(w, r)
	}

	// /api/places/{id} - 获取详情
	if len(parts) == 1 {
		return middleware.OptionalAuthMiddlewareHTTP(handler.GetPlaceDetail)(w, r)
	}

	return nil
}

// handlePhotoRoutes 处理照片相关动态路由
func handlePhotoRoutes(w http.ResponseWriter, r *http.Request) error {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/photos/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		return nil
	}

	// /api/photos/{id}/like
	if len(parts) >= 2 && parts[1] == "like" {
		return middleware.AuthMiddlewareHTTP(handler.LikePhoto)(w, r)
	}

	// /api/photos/{id}/comments
	if len(parts) >= 2 && parts[1] == "comments" {
		if r.Method == http.MethodPost {
			return middleware.AuthMiddlewareHTTP(handler.PostComment)(w, r)
		}
		return middleware.OptionalAuthMiddlewareHTTP(handler.GetPhotoComments)(w, r)
	}

	// /api/photos/{id} - 获取详情
	if len(parts) == 1 {
		return middleware.OptionalAuthMiddlewareHTTP(handler.GetPhotoDetail)(w, r)
	}

	return nil
}

// 包装函数类型适配
type httpHandler func(http.ResponseWriter, *http.Request) error

func wrapHandler(h httpHandler) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		middleware.CORSMiddlewareHTTP(func(w http.ResponseWriter, r *http.Request) error {
			return h(w, r)
		})(w, r)
		return nil
	}
}

func wrapAuth(h httpHandler) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		return middleware.CORSMiddlewareHTTP(
			middleware.AuthMiddlewareHTTP(h),
		)(w, r)
	}
}

func wrapOptionalAuth(h httpHandler) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		return middleware.CORSMiddlewareHTTP(
			middleware.OptionalAuthMiddlewareHTTP(h),
		)(w, r)
	}
}
