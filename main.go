package main

import (
	"net/http"

	trpc "trpc.group/trpc-go/trpc-go"
	thttp "trpc.group/trpc-go/trpc-go/http"
	"trpc.group/trpc-go/trpc-go/log"

	_ "timespace/config" // 触发 init() 注册自定义插件
	"timespace/db"
	"timespace/handler"
	"timespace/middleware"
	"timespace/router"
)

func main() {
	s := trpc.NewServer()

	// 初始化数据库
	if err := db.InitMySQL(); err != nil {
		log.Errorf("[INIT] mysql connect failed: %v (服务将以无DB模式运行)", err)
	} else {
		defer db.CloseMySQL()
		log.Info("[INIT] mysql connected")
	}
	if err := db.InitRedis(); err != nil {
		log.Errorf("[INIT] redis connect failed: %v (服务将以无缓存模式运行)", err)
	} else {
		defer db.CloseRedis()
		log.Info("[INIT] redis connected")
	}

	// ============ 路由注册（支持 :id 动态参数） ============
	r := router.New()

	// --- 用户 ---
	r.POST("/api/user/login", toStdHandler(wrapPlain(handler.UserLogin)))
	r.GET("/api/user/info", toStdHandler(wrapAuth(handler.GetUserInfo)))
	r.POST("/api/user/update", toStdHandler(wrapAuth(handler.UpdateUserInfo)))
	r.GET("/api/user/stats", toStdHandler(wrapAuth(handler.GetUserStats)))
	r.GET("/api/user/achievements", toStdHandler(wrapAuth(handler.GetUserAchievements)))
	r.GET("/api/user/photos", toStdHandler(wrapAuth(handler.GetUserPhotos)))
	r.GET("/api/user/favorites", toStdHandler(wrapAuth(handler.GetUserFavorites)))
	r.GET("/api/user/footprints", toStdHandler(wrapAuth(handler.GetUserFootprints)))

	// --- 记忆点 ---
	r.GET("/api/places/nearby", toStdHandler(wrapOptional(handler.GetNearbyPlaces)))
	r.GET("/api/places/search", toStdHandler(wrapOptional(handler.SearchPlaces)))
	r.POST("/api/places/create", toStdHandler(wrapAuth(handler.CreatePlace)))
	r.GET("/api/places/:id", toStdHandler(wrapOptional(handler.GetPlaceDetail)))
	r.GET("/api/places/:id/photos", toStdHandler(wrapOptional(handler.GetPlacePhotos)))

	// --- 照片 ---
	r.POST("/api/photos/publish", toStdHandler(wrapAuth(handler.PublishPhotos)))
	r.GET("/api/photos/:id", toStdHandler(wrapOptional(handler.GetPhotoDetail)))
	r.POST("/api/photos/:id/like", toStdHandler(wrapAuth(handler.LikePhoto)))
	r.GET("/api/photos/:id/comments", toStdHandler(wrapOptional(handler.GetPhotoComments)))
	r.POST("/api/photos/:id/comments", toStdHandler(wrapAuth(handler.PostComment)))

	// --- 文件上传 / 收藏 / 品牌 ---
	r.POST("/api/upload", toStdHandler(wrapAuth(handler.UploadFile)))
	r.POST("/api/favorite", toStdHandler(wrapAuth(handler.FavoritePhoto)))
	r.GET("/api/brand/memories", toStdHandler(wrapOptional(handler.GetBrandMemories)))

	// --- 健康检查 ---
	r.GET("/health", func(w http.ResponseWriter, req *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"status":"healthy"}}`))
		return nil
	})

	// --- 静态文件服务 ---
	r.ANY("/static/.*", func(w http.ResponseWriter, req *http.Request) error {
		http.StripPrefix("/static/", http.FileServer(http.Dir("."))).ServeHTTP(w, req)
		return nil
	})

	// 注册整个 router 作为 mux：tRPC 用 "*" 通配方法接收所有请求并转发给 r
	thttp.RegisterNoProtocolServiceMux(s.Service("trpc.timespace.capsule.http"), r)

	log.Info("[INIT] timespace server starting on :8080 ...")
	if err := s.Serve(); err != nil {
		log.Fatalf("[FATAL] server serve failed: %v", err)
	}
}

// toStdHandler 把返回 error 的 HTTPHandler 转为标准 http handler（适配 router）
func toStdHandler(h middleware.HTTPHandler) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		return h(w, r)
	}
}

// ============ 中间件包装 ============

func wrapPlain(h middleware.HTTPHandler) middleware.HTTPHandler {
	return middleware.RecoveryMiddlewareHTTP(
		middleware.CORSMiddlewareHTTP(
			middleware.AccessLogMiddlewareHTTP(h),
		),
	)
}

func wrapAuth(h middleware.HTTPHandler) middleware.HTTPHandler {
	return middleware.RecoveryMiddlewareHTTP(
		middleware.CORSMiddlewareHTTP(
			middleware.AccessLogMiddlewareHTTP(
				middleware.AuthMiddlewareHTTP(h),
			),
		),
	)
}

func wrapOptional(h middleware.HTTPHandler) middleware.HTTPHandler {
	return middleware.RecoveryMiddlewareHTTP(
		middleware.CORSMiddlewareHTTP(
			middleware.AccessLogMiddlewareHTTP(
				middleware.OptionalAuthMiddlewareHTTP(h),
			),
		),
	)
}
