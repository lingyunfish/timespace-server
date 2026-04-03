package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	trpc "trpc.group/trpc-go/trpc-go"

	"timespace/db"
	"timespace/middleware"
	"timespace/model"
	"timespace/util"
)

// ---- sqlx DB 映射结构体 ----

type PlaceDB struct {
	ID           uint64  `db:"id"`
	Name         string  `db:"name"`
	Description  string  `db:"description"`
	Latitude     float64 `db:"latitude"`
	Longitude    float64 `db:"longitude"`
	Address      string  `db:"address"`
	City         string  `db:"city"`
	Province     string  `db:"province"`
	CoverURL     string  `db:"cover_url"`
	PhotoCount   int     `db:"photo_count"`
	VisitorCount int     `db:"visitor_count"`
	LikeCount    int     `db:"like_count"`
	IsOfficial   int     `db:"is_official"`
	Category     string  `db:"category"`
	CreatorID    uint64  `db:"creator_id"`
	CreatedAt    string  `db:"created_at"`
}

type PhotoDB struct {
	ID           uint64  `db:"id"`
	UserID       uint64  `db:"user_id"`
	PlaceID      uint64  `db:"place_id"`
	ImageURL     string  `db:"image_url"`
	ThumbnailURL string  `db:"thumbnail_url"`
	Description  string  `db:"description"`
	Latitude     float64 `db:"latitude"`
	Longitude    float64 `db:"longitude"`
	LikeCount    int     `db:"like_count"`
	CommentCount int     `db:"comment_count"`
	ViewCount    int     `db:"view_count"`
	IsPreview    int     `db:"is_preview"`
	CreatedAt    string  `db:"created_at"`
	UserName     string  `db:"nickname"`
	UserAvatar   string  `db:"avatar_url"`
	PlaceName    string  `db:"place_name"`
}

type CommentDB struct {
	ID         uint64 `db:"id"`
	Content    string `db:"content"`
	CreatedAt  string `db:"created_at"`
	UserName   string `db:"nickname"`
	UserAvatar string `db:"avatar_url"`
}

type PhotoThumbDB struct {
	ID           uint64 `db:"id"`
	ThumbnailURL string `db:"thumbnail_url"`
}

type PhotoImageDB struct {
	ID       uint64 `db:"id"`
	ImageURL string `db:"image_url"`
}

type UserPhotoListDB struct {
	ID           uint64 `db:"id"`
	ImageURL     string `db:"image_url"`
	ThumbnailURL string `db:"thumbnail_url"`
	Description  string `db:"description"`
	LikeCount    int    `db:"like_count"`
	CommentCount int    `db:"comment_count"`
	CreatedAt    string `db:"created_at"`
	PlaceName    string `db:"place_name"`
}

type UserFavListDB struct {
	ID           uint64 `db:"id"`
	ImageURL     string `db:"image_url"`
	ThumbnailURL string `db:"thumbnail_url"`
	Description  string `db:"description"`
	LikeCount    int    `db:"like_count"`
	CreatedAt    string `db:"created_at"`
	PlaceName    string `db:"place_name"`
}

type FootprintDB struct {
	ID           uint64  `db:"id"`
	Name         string  `db:"name"`
	Latitude     float64 `db:"latitude"`
	Longitude    float64 `db:"longitude"`
	City         string  `db:"city"`
	PhotoCount   int     `db:"photo_count"`
	VisitCount   int     `db:"visit_count"`
	FirstVisitAt string  `db:"first_visit_at"`
	LastVisitAt  string  `db:"last_visit_at"`
}

type AchievementCheckDB struct {
	ID             uint64 `db:"id"`
	ConditionType  string `db:"condition_type"`
	ConditionValue int    `db:"condition_value"`
	ExpReward      int    `db:"exp_reward"`
}

// GetNearbyPlaces 获取附近记忆点
func GetNearbyPlaces(w http.ResponseWriter, r *http.Request) error {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("latitude"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("longitude"), 64)
	radius, _ := strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
	if lat == 0 || lng == 0 {
		util.Error(w, 400, "经纬度参数不能为空")
		return nil
	}
	if radius == 0 {
		radius = 5000
	}

	delta := radius / 111000.0
	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var rows []PlaceDB
	err := proxy.Select(ctx, &rows,
		`SELECT id, name, COALESCE(description,'') as description, latitude, longitude, COALESCE(address,'') as address, COALESCE(cover_url,'') as cover_url,
			photo_count, visitor_count, like_count, is_official, COALESCE(category,'') as category
		FROM places WHERE status = 1
			AND latitude BETWEEN ? AND ?
			AND longitude BETWEEN ? AND ?
		ORDER BY photo_count DESC LIMIT 100`,
		lat-delta, lat+delta, lng-delta, lng+delta,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}

	var places []model.Place
	for _, row := range rows {
		dist := util.CalcDistance(lat, lng, row.Latitude, row.Longitude)
		if dist <= radius {
			p := model.Place{
				ID: row.ID, Name: row.Name, Description: row.Description,
				Latitude: row.Latitude, Longitude: row.Longitude, Address: row.Address,
				CoverURL: row.CoverURL, PhotoCount: row.PhotoCount,
				VisitorCount: row.VisitorCount, LikeCount: row.LikeCount,
				IsOfficial: row.IsOfficial != 0, Category: row.Category,
				Distance: dist, DistanceText: util.FormatDistance(dist),
			}
			// 加载预览照片
			var thumbs []PhotoThumbDB
			proxy.Select(ctx, &thumbs,
				`SELECT id, thumbnail_url FROM photos WHERE place_id = ? AND status = 1 ORDER BY created_at DESC LIMIT 5`, row.ID)
			for _, t := range thumbs {
				p.Photos = append(p.Photos, model.Photo{ID: t.ID, ThumbnailURL: t.ThumbnailURL})
			}
			places = append(places, p)
		}
	}

	util.Success(w, map[string]interface{}{"places": places})
	return nil
}

// SearchPlaces 搜索记忆点
func SearchPlaces(w http.ResponseWriter, r *http.Request) error {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		util.Error(w, 400, "搜索关键词不能为空")
		return nil
	}

	proxy := db.GetMySQLProxy()
	var rows []PlaceDB
	like := "%" + keyword + "%"
	err := proxy.Select(r.Context(), &rows,
		`SELECT id, name, COALESCE(description,'') as description, latitude, longitude, COALESCE(address,'') as address, COALESCE(city,'') as city, photo_count, is_official
		FROM places WHERE status = 1 AND (name LIKE ? OR address LIKE ? OR city LIKE ?)
		ORDER BY photo_count DESC LIMIT 20`, like, like, like)
	if err != nil {
		util.Error(w, 500, "搜索失败")
		return nil
	}

	var places []model.Place
	for _, row := range rows {
		places = append(places, model.Place{
			ID: row.ID, Name: row.Name, Description: row.Description,
			Latitude: row.Latitude, Longitude: row.Longitude, Address: row.Address,
			City: row.City, PhotoCount: row.PhotoCount, IsOfficial: row.IsOfficial != 0,
		})
	}
	util.Success(w, map[string]interface{}{"places": places})
	return nil
}

// GetPlaceDetail 获取记忆点详情
func GetPlaceDetail(w http.ResponseWriter, r *http.Request) error {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	placeID, _ := strconv.ParseUint(parts[3], 10, 64)
	if placeID == 0 {
		util.Error(w, 400, "地点ID无效")
		return nil
	}

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var row PlaceDB
	err := proxy.QueryToStruct(ctx, &row,
		`SELECT id, name, COALESCE(description,'') as description, latitude, longitude, COALESCE(address,'') as address,
			COALESCE(city,'') as city, COALESCE(province,'') as province, COALESCE(cover_url,'') as cover_url,
			photo_count, visitor_count, like_count, is_official, COALESCE(category,'') as category, creator_id, created_at
		FROM places WHERE id = ? AND status = 1`, placeID)
	if err != nil {
		util.Error(w, 404, "地点不存在")
		return nil
	}

	place := model.Place{
		ID: row.ID, Name: row.Name, Description: row.Description,
		Latitude: row.Latitude, Longitude: row.Longitude, Address: row.Address,
		City: row.City, Province: row.Province, CoverURL: row.CoverURL,
		PhotoCount: row.PhotoCount, VisitorCount: row.VisitorCount, LikeCount: row.LikeCount,
		IsOfficial: row.IsOfficial != 0, Category: row.Category, CreatorID: row.CreatorID,
	}

	userID := middleware.GetUserID(ctx)
	if userID > 0 {
		proxy.Exec(ctx,
			`INSERT INTO footprints (user_id, place_id) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE visit_count = visit_count + 1, last_visit_at = NOW()`, userID, placeID)
		proxy.Exec(ctx,
			`UPDATE places SET visitor_count = (SELECT COUNT(DISTINCT user_id) FROM footprints WHERE place_id = ?) WHERE id = ?`, placeID, placeID)
	}

	util.Success(w, place)
	return nil
}

// CreatePlace 创建记忆点
func CreatePlace(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Address     string  `json:"address"`
		City        string  `json:"city"`
		Province    string  `json:"province"`
		Category    string  `json:"category"`
	}
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}
	if req.Name == "" || req.Latitude == 0 || req.Longitude == 0 {
		util.Error(w, 400, "名称和经纬度不能为空")
		return nil
	}

	proxy := db.GetMySQLProxy()
	ctx := r.Context()
	delta := 0.001
	var existID uint64
	proxy.QueryRow(ctx, []interface{}{&existID},
		`SELECT id FROM places WHERE name = ? AND status = 1
		AND latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ? LIMIT 1`,
		req.Name, req.Latitude-delta, req.Latitude+delta, req.Longitude-delta, req.Longitude+delta)

	if existID > 0 {
		util.Error(w, 409, "附近已存在同名记忆点")
		return nil
	}

	result, err := proxy.Exec(ctx,
		`INSERT INTO places (name, description, latitude, longitude, address, city, province, category, creator_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Description, req.Latitude, req.Longitude, req.Address, req.City, req.Province, req.Category, userID)
	if err != nil {
		util.Error(w, 500, "创建失败")
		return nil
	}
	id, _ := result.LastInsertId()
	util.Success(w, map[string]interface{}{"id": id})
	return nil
}

// GetPlacePhotos 获取记忆点的照片列表
func GetPlacePhotos(w http.ResponseWriter, r *http.Request) error {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	placeID, _ := strconv.ParseUint(parts[3], 10, 64)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	sort := r.URL.Query().Get("sort")
	if page <= 0 { page = 1 }
	if pageSize <= 0 || pageSize > 50 { pageSize = 20 }

	orderBy := "p.created_at DESC"
	switch sort {
	case "popular":
		orderBy = "p.like_count DESC"
	case "oldest":
		orderBy = "p.created_at ASC"
	}
	offset := (page - 1) * pageSize
	userID := middleware.GetUserID(r.Context())

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var rows []PhotoDB
	query := fmt.Sprintf(
		`SELECT p.id, p.user_id, p.image_url, p.thumbnail_url, COALESCE(p.description,'') as description,
			p.latitude, p.longitude, p.like_count, p.comment_count, p.is_preview,
			p.created_at, COALESCE(u.nickname,'') as nickname, COALESCE(u.avatar_url,'') as avatar_url
		FROM photos p LEFT JOIN users u ON u.id = p.user_id
		WHERE p.place_id = ? AND p.status = 1 ORDER BY %s LIMIT ? OFFSET ?`, orderBy)
	err := proxy.Select(ctx, &rows, query, placeID, pageSize, offset)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}

	var photos []model.Photo
	for _, row := range rows {
		p := model.Photo{
			ID: row.ID, UserID: row.UserID, ImageURL: row.ImageURL,
			ThumbnailURL: row.ThumbnailURL, Description: row.Description,
			Latitude: row.Latitude, Longitude: row.Longitude,
			LikeCount: row.LikeCount, CommentCount: row.CommentCount,
			IsPreview: row.IsPreview != 0, UserName: row.UserName, UserAvatar: row.UserAvatar,
		}
		if userID > 0 {
			var likeID uint64
			proxy.QueryRow(ctx, []interface{}{&likeID},
				"SELECT id FROM likes WHERE user_id = ? AND photo_id = ?", userID, row.ID)
			p.IsLiked = likeID > 0
		}
		photos = append(photos, p)
	}
	util.Success(w, map[string]interface{}{"photos": photos})
	return nil
}

// PublishPhotos 发布照片
func PublishPhotos(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	var req struct {
		PlaceID     uint64   `json:"place_id"`
		PlaceName   string   `json:"place_name"`
		Latitude    float64  `json:"latitude"`
		Longitude   float64  `json:"longitude"`
		Description string   `json:"description"`
		ImageURLs   []string `json:"image_urls"`
	}
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}
	if len(req.ImageURLs) == 0 {
		util.Error(w, 400, "至少需要一张照片")
		return nil
	}
	if req.Latitude == 0 || req.Longitude == 0 {
		util.Error(w, 400, "位置信息无效")
		return nil
	}

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	placeID := req.PlaceID
	if placeID == 0 {
		delta := 0.0005
		var existID uint64
		proxy.QueryRow(ctx, []interface{}{&existID},
			`SELECT id FROM places WHERE status = 1
			AND latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ? LIMIT 1`,
			req.Latitude-delta, req.Latitude+delta, req.Longitude-delta, req.Longitude+delta)
		if existID > 0 {
			placeID = existID
		} else {
			name := req.PlaceName
			if name == "" { name = "记忆点" }
			result, err := proxy.Exec(ctx,
				`INSERT INTO places (name, latitude, longitude, creator_id) VALUES (?, ?, ?, ?)`,
				name, req.Latitude, req.Longitude, userID)
			if err != nil {
				util.Error(w, 500, "创建地点失败")
				return nil
			}
			id, _ := result.LastInsertId()
			placeID = uint64(id)
		}
	}

	var placeLat, placeLng float64
	proxy.QueryRow(ctx, []interface{}{&placeLat, &placeLng},
		"SELECT latitude, longitude FROM places WHERE id = ?", placeID)
	dist := util.CalcDistance(req.Latitude, req.Longitude, placeLat, placeLng)
	if dist > 200 {
		util.Error(w, 403, "你不在记忆点附近，无法投递")
		return nil
	}

	var photoIDs []int64
	for i, imgURL := range req.ImageURLs {
		isPreview := 0
		if i < 3 { isPreview = 1 }
		result, err := proxy.Exec(ctx,
			`INSERT INTO photos (user_id, place_id, image_url, thumbnail_url, description, latitude, longitude, is_preview)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, placeID, imgURL, imgURL, req.Description, req.Latitude, req.Longitude, isPreview)
		if err != nil { continue }
		id, _ := result.LastInsertId()
		photoIDs = append(photoIDs, id)
	}

	proxy.Exec(ctx,
		"UPDATE places SET photo_count = (SELECT COUNT(*) FROM photos WHERE place_id = ? AND status = 1) WHERE id = ?",
		placeID, placeID)
	proxy.Exec(ctx,
		`INSERT INTO footprints (user_id, place_id) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE visit_count = visit_count + 1, last_visit_at = NOW()`, userID, placeID)

	go checkAchievements(userID)

	util.Success(w, map[string]interface{}{"photo_ids": photoIDs, "place_id": placeID})
	return nil
}

// GetPhotoDetail 获取照片详情
func GetPhotoDetail(w http.ResponseWriter, r *http.Request) error {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	photoID, _ := strconv.ParseUint(parts[3], 10, 64)

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var row PhotoDB
	err := proxy.QueryToStruct(ctx, &row,
		`SELECT p.id, p.user_id, p.place_id, p.image_url, p.thumbnail_url, COALESCE(p.description,'') as description,
			p.latitude, p.longitude, p.like_count, p.comment_count, p.view_count,
			p.created_at, COALESCE(u.nickname,'') as nickname, COALESCE(u.avatar_url,'') as avatar_url, COALESCE(pl.name,'') as place_name
		FROM photos p LEFT JOIN users u ON u.id = p.user_id LEFT JOIN places pl ON pl.id = p.place_id
		WHERE p.id = ? AND p.status = 1`, photoID)
	if err != nil {
		util.Error(w, 404, "照片不存在")
		return nil
	}

	proxy.Exec(ctx, "UPDATE photos SET view_count = view_count + 1 WHERE id = ?", photoID)

	isLiked := false
	userID := middleware.GetUserID(ctx)
	if userID > 0 {
		var likeID uint64
		proxy.QueryRow(ctx, []interface{}{&likeID},
			"SELECT id FROM likes WHERE user_id = ? AND photo_id = ?", userID, photoID)
		isLiked = likeID > 0
	}

	var imgs []PhotoImageDB
	proxy.Select(ctx, &imgs,
		`SELECT id, image_url FROM photos WHERE place_id = ? AND status = 1 ORDER BY created_at DESC LIMIT 20`, row.PlaceID)

	var images []map[string]interface{}
	for _, img := range imgs {
		images = append(images, map[string]interface{}{"id": img.ID, "image_url": img.ImageURL})
	}

	util.Success(w, map[string]interface{}{
		"id": row.ID, "user_id": row.UserID, "place_id": row.PlaceID,
		"image_url": row.ImageURL, "thumbnail_url": row.ThumbnailURL,
		"description": row.Description, "latitude": row.Latitude, "longitude": row.Longitude,
		"like_count": row.LikeCount, "comment_count": row.CommentCount, "view_count": row.ViewCount,
		"created_at": row.CreatedAt, "user_name": row.UserName, "user_avatar": row.UserAvatar,
		"place_name": row.PlaceName, "is_liked": isLiked, "images": images,
	})
	return nil
}

// LikePhoto 点赞/取消点赞
func LikePhoto(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	photoID, _ := strconv.ParseUint(parts[3], 10, 64)
	var req struct{ Action string `json:"action"` }
	util.ParseJSON(r, &req)

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	if req.Action == "unlike" {
		proxy.Exec(ctx, "DELETE FROM likes WHERE user_id = ? AND photo_id = ?", userID, photoID)
		proxy.Exec(ctx, "UPDATE photos SET like_count = GREATEST(like_count - 1, 0) WHERE id = ?", photoID)
	} else {
		proxy.Exec(ctx, "INSERT IGNORE INTO likes (user_id, photo_id) VALUES (?, ?)", userID, photoID)
		proxy.Exec(ctx, "UPDATE photos SET like_count = like_count + 1 WHERE id = ?", photoID)
		var placeID uint64
		proxy.QueryRow(ctx, []interface{}{&placeID}, "SELECT place_id FROM photos WHERE id = ?", photoID)
		if placeID > 0 {
			proxy.Exec(ctx,
				`UPDATE places SET like_count = (SELECT COALESCE(SUM(like_count), 0) FROM photos WHERE place_id = ? AND status = 1) WHERE id = ?`,
				placeID, placeID)
		}
	}
	util.Success(w, nil)
	return nil
}

// GetPhotoComments 获取照片评论
func GetPhotoComments(w http.ResponseWriter, r *http.Request) error {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	photoID, _ := strconv.ParseUint(parts[3], 10, 64)

	proxy := db.GetMySQLProxy()
	var rows []CommentDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT c.id, c.content, c.created_at, COALESCE(u.nickname,'') as nickname, COALESCE(u.avatar_url,'') as avatar_url
		FROM comments c LEFT JOIN users u ON u.id = c.user_id
		WHERE c.photo_id = ? AND c.status = 1 ORDER BY c.created_at DESC LIMIT 50`, photoID)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	var comments []map[string]interface{}
	for _, row := range rows {
		comments = append(comments, map[string]interface{}{
			"id": row.ID, "content": row.Content, "created_at": row.CreatedAt,
			"user_name": row.UserName, "user_avatar": row.UserAvatar,
		})
	}
	util.Success(w, map[string]interface{}{"comments": comments})
	return nil
}

// PostComment 发表评论
func PostComment(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		util.Error(w, 400, "参数错误")
		return nil
	}
	photoID, _ := strconv.ParseUint(parts[3], 10, 64)
	var req struct {
		Content   string  `json:"content"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}
	if req.Content == "" || len(req.Content) > 500 {
		util.Error(w, 400, "评论内容无效")
		return nil
	}

	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var exists int
	proxy.QueryRow(ctx, []interface{}{&exists}, "SELECT 1 FROM photos WHERE id = ? AND status = 1", photoID)
	if exists == 0 {
		util.Error(w, 404, "照片不存在")
		return nil
	}

	result, err := proxy.Exec(ctx,
		"INSERT INTO comments (photo_id, user_id, content, latitude, longitude) VALUES (?, ?, ?, ?, ?)",
		photoID, userID, req.Content, req.Latitude, req.Longitude)
	if err != nil {
		util.Error(w, 500, "发表评论失败")
		return nil
	}
	proxy.Exec(ctx, "UPDATE photos SET comment_count = comment_count + 1 WHERE id = ?", photoID)

	id, _ := result.LastInsertId()
	util.Success(w, map[string]interface{}{"id": id})
	return nil
}

// FavoritePhoto 收藏/取消收藏
func FavoritePhoto(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	var req struct {
		PhotoID uint64 `json:"photo_id"`
		Action  string `json:"action"`
	}
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}
	proxy := db.GetMySQLProxy()
	if req.Action == "remove" {
		proxy.Exec(r.Context(), "DELETE FROM favorites WHERE user_id = ? AND photo_id = ?", userID, req.PhotoID)
	} else {
		proxy.Exec(r.Context(), "INSERT IGNORE INTO favorites (user_id, photo_id) VALUES (?, ?)", userID, req.PhotoID)
	}
	util.Success(w, nil)
	return nil
}

// GetUserPhotos 获取用户照片列表
func GetUserPhotos(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page <= 0 { page = 1 }
	if pageSize <= 0 || pageSize > 50 { pageSize = 20 }
	offset := (page - 1) * pageSize

	proxy := db.GetMySQLProxy()
	var rows []UserPhotoListDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT p.id, p.image_url, p.thumbnail_url, COALESCE(p.description,'') as description,
			p.like_count, p.comment_count, p.created_at, COALESCE(pl.name,'') as place_name
		FROM photos p LEFT JOIN places pl ON pl.id = p.place_id
		WHERE p.user_id = ? AND p.status = 1 ORDER BY p.created_at DESC LIMIT ? OFFSET ?`,
		userID, pageSize, offset)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	util.Success(w, map[string]interface{}{"photos": rows})
	return nil
}

// GetUserFavorites 获取用户收藏列表
func GetUserFavorites(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page <= 0 { page = 1 }
	if pageSize <= 0 || pageSize > 50 { pageSize = 20 }
	offset := (page - 1) * pageSize

	proxy := db.GetMySQLProxy()
	var rows []UserFavListDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT p.id, p.image_url, p.thumbnail_url, COALESCE(p.description,'') as description,
			p.like_count, p.created_at, COALESCE(pl.name,'') as place_name
		FROM favorites f JOIN photos p ON p.id = f.photo_id AND p.status = 1
		LEFT JOIN places pl ON pl.id = p.place_id
		WHERE f.user_id = ? ORDER BY f.created_at DESC LIMIT ? OFFSET ?`,
		userID, pageSize, offset)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	util.Success(w, map[string]interface{}{"photos": rows})
	return nil
}

// GetUserFootprints 获取用户足迹
func GetUserFootprints(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	proxy := db.GetMySQLProxy()
	var rows []FootprintDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT p.id, p.name, p.latitude, p.longitude, COALESCE(p.city,'') as city, p.photo_count,
			f.visit_count, f.first_visit_at, f.last_visit_at
		FROM footprints f JOIN places p ON p.id = f.place_id
		WHERE f.user_id = ? ORDER BY f.last_visit_at DESC`, userID)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	util.Success(w, map[string]interface{}{"footprints": rows})
	return nil
}

// checkAchievements 检查并解锁成就（异步）
func checkAchievements(userID uint64) {
	proxy := db.GetMySQLProxy()
	if proxy == nil {
		return
	}
	ctx := trpc.BackgroundContext()

	var photoCount, placeCount, likeReceived, commentCount, cityCount int
	proxy.QueryRow(ctx, []interface{}{&photoCount}, "SELECT COUNT(*) FROM photos WHERE user_id = ? AND status = 1", userID)
	proxy.QueryRow(ctx, []interface{}{&placeCount}, "SELECT COUNT(*) FROM footprints WHERE user_id = ?", userID)
	proxy.QueryRow(ctx, []interface{}{&likeReceived}, "SELECT COALESCE(SUM(like_count), 0) FROM photos WHERE user_id = ? AND status = 1", userID)
	proxy.QueryRow(ctx, []interface{}{&commentCount}, "SELECT COUNT(*) FROM comments WHERE user_id = ? AND status = 1", userID)
	proxy.QueryRow(ctx, []interface{}{&cityCount},
		`SELECT COUNT(DISTINCT p.city) FROM footprints f JOIN places p ON p.id = f.place_id WHERE f.user_id = ?`, userID)

	var achRows []AchievementCheckDB
	proxy.Select(ctx, &achRows,
		`SELECT ad.id, ad.condition_type, ad.condition_value, ad.exp_reward
		FROM achievement_defs ad
		WHERE ad.status = 1 AND ad.id NOT IN (SELECT achievement_id FROM user_achievements WHERE user_id = ?)`, userID)

	for _, ach := range achRows {
		var current int
		switch ach.ConditionType {
		case "photo_count":
			current = photoCount
		case "place_count":
			current = placeCount
		case "like_received":
			current = likeReceived
		case "comment_count":
			current = commentCount
		case "city_count":
			current = cityCount
		}
		if current >= ach.ConditionValue {
			proxy.Exec(ctx, "INSERT IGNORE INTO user_achievements (user_id, achievement_id) VALUES (?, ?)", userID, ach.ID)
			proxy.Exec(ctx, "UPDATE users SET exp = exp + ?, updated_at = NOW() WHERE id = ?", ach.ExpReward, userID)
			var totalExp int
			proxy.QueryRow(ctx, []interface{}{&totalExp}, "SELECT exp FROM users WHERE id = ?", userID)
			newLevel := 1
			if totalExp >= 200 {
				newLevel = 6
			} else if totalExp >= 150 {
				newLevel = 5
			} else if totalExp >= 100 {
				newLevel = 4
			} else if totalExp >= 50 {
				newLevel = 3
			} else if totalExp >= 20 {
				newLevel = 2
			}
			proxy.Exec(ctx, "UPDATE users SET level = ?, updated_at = NOW() WHERE id = ? AND level < ?", newLevel, userID, newLevel)
		}
	}
}
