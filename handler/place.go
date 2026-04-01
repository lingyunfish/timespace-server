package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"timespace/db"
	"timespace/middleware"
	"timespace/model"
	"timespace/util"
)

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

	// 使用经纬度范围初筛，再精确计算距离
	// 经度1度 ≈ 111km * cos(lat), 纬度1度 ≈ 111km
	delta := radius / 111000.0
	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT id, name, description, latitude, longitude, address, cover_url,
			photo_count, visitor_count, like_count, is_official, category
		FROM places
		WHERE status = 1
			AND latitude BETWEEN ? AND ?
			AND longitude BETWEEN ? AND ?
		ORDER BY photo_count DESC
		LIMIT 100`,
		lat-delta, lat+delta, lng-delta, lng+delta,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var places []model.Place
	for rows.Next() {
		var p model.Place
		rows.Scan(&p.ID, &p.Name, &p.Description, &p.Latitude, &p.Longitude,
			&p.Address, &p.CoverURL, &p.PhotoCount, &p.VisitorCount,
			&p.LikeCount, &p.IsOfficial, &p.Category)
		p.Distance = util.CalcDistance(lat, lng, p.Latitude, p.Longitude)
		if p.Distance <= radius {
			p.DistanceText = util.FormatDistance(p.Distance)
			places = append(places, p)
		}
	}

	// 为每个地点加载预览照片
	for i := range places {
		photoRows, err := mysql.QueryContext(r.Context(),
			`SELECT id, thumbnail_url FROM photos WHERE place_id = ? AND status = 1 ORDER BY created_at DESC LIMIT 5`,
			places[i].ID,
		)
		if err == nil {
			var photos []model.Photo
			for photoRows.Next() {
				var photo model.Photo
				photoRows.Scan(&photo.ID, &photo.ThumbnailURL)
				photos = append(photos, photo)
			}
			photoRows.Close()
			places[i].Photos = photos
		}
	}

	util.Success(w, map[string]interface{}{
		"places": places,
	})
	return nil
}

// SearchPlaces 搜索记忆点
func SearchPlaces(w http.ResponseWriter, r *http.Request) error {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		util.Error(w, 400, "搜索关键词不能为空")
		return nil
	}

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT id, name, description, latitude, longitude, address, city, photo_count, is_official
		FROM places
		WHERE status = 1 AND (name LIKE ? OR address LIKE ? OR city LIKE ?)
		ORDER BY photo_count DESC
		LIMIT 20`,
		"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
	)
	if err != nil {
		util.Error(w, 500, "搜索失败")
		return nil
	}
	defer rows.Close()

	var places []model.Place
	for rows.Next() {
		var p model.Place
		rows.Scan(&p.ID, &p.Name, &p.Description, &p.Latitude, &p.Longitude,
			&p.Address, &p.City, &p.PhotoCount, &p.IsOfficial)
		places = append(places, p)
	}

	util.Success(w, map[string]interface{}{
		"places": places,
	})
	return nil
}

// GetPlaceDetail 获取记忆点详情
func GetPlaceDetail(w http.ResponseWriter, r *http.Request) error {
	// 从URL提取place_id: /api/places/{id}
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

	mysql := db.GetMySQL()
	var place model.Place
	err := mysql.QueryRowContext(r.Context(),
		`SELECT id, name, description, latitude, longitude, address, city, province, cover_url,
			photo_count, visitor_count, like_count, is_official, category, creator_id, created_at
		FROM places WHERE id = ? AND status = 1`, placeID,
	).Scan(&place.ID, &place.Name, &place.Description, &place.Latitude, &place.Longitude,
		&place.Address, &place.City, &place.Province, &place.CoverURL,
		&place.PhotoCount, &place.VisitorCount, &place.LikeCount,
		&place.IsOfficial, &place.Category, &place.CreatorID, &place.CreatedAt)
	if err != nil {
		util.Error(w, 404, "地点不存在")
		return nil
	}

	// 记录足迹
	userID := middleware.GetUserID(r.Context())
	if userID > 0 {
		mysql.ExecContext(r.Context(),
			`INSERT INTO footprints (user_id, place_id) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE visit_count = visit_count + 1, last_visit_at = NOW()`,
			userID, placeID,
		)
		// 更新访客数
		mysql.ExecContext(r.Context(),
			`UPDATE places SET visitor_count = (SELECT COUNT(DISTINCT user_id) FROM footprints WHERE place_id = ?) WHERE id = ?`,
			placeID, placeID,
		)
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

	// 检查附近是否已有同名地点
	mysql := db.GetMySQL()
	var existID uint64
	delta := 0.001 // ~100m范围
	mysql.QueryRowContext(r.Context(),
		`SELECT id FROM places WHERE name = ? AND status = 1
		AND latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ? LIMIT 1`,
		req.Name, req.Latitude-delta, req.Latitude+delta, req.Longitude-delta, req.Longitude+delta,
	).Scan(&existID)

	if existID > 0 {
		util.Error(w, 409, "附近已存在同名记忆点")
		return nil
	}

	result, err := mysql.ExecContext(r.Context(),
		`INSERT INTO places (name, description, latitude, longitude, address, city, province, category, creator_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Description, req.Latitude, req.Longitude,
		req.Address, req.City, req.Province, req.Category, userID,
	)
	if err != nil {
		util.Error(w, 500, "创建失败")
		return nil
	}

	id, _ := result.LastInsertId()
	util.Success(w, map[string]interface{}{
		"id": id,
	})
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

	mysql := db.GetMySQL()
	query := fmt.Sprintf(
		`SELECT p.id, p.user_id, p.image_url, p.thumbnail_url, p.description,
			p.latitude, p.longitude, p.like_count, p.comment_count, p.is_preview,
			p.created_at, u.nickname, u.avatar_url
		FROM photos p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.place_id = ? AND p.status = 1
		ORDER BY %s
		LIMIT ? OFFSET ?`, orderBy)

	rows, err := mysql.QueryContext(r.Context(), query, placeID, pageSize, offset)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var photos []model.Photo
	for rows.Next() {
		var p model.Photo
		rows.Scan(&p.ID, &p.UserID, &p.ImageURL, &p.ThumbnailURL, &p.Description,
			&p.Latitude, &p.Longitude, &p.LikeCount, &p.CommentCount, &p.IsPreview,
			&p.CreatedAt, &p.UserName, &p.UserAvatar)

		// 检查是否点赞
		if userID > 0 {
			var likeID uint64
			mysql.QueryRowContext(r.Context(),
				"SELECT id FROM likes WHERE user_id = ? AND photo_id = ?", userID, p.ID,
			).Scan(&likeID)
			p.IsLiked = likeID > 0
		}
		photos = append(photos, p)
	}

	util.Success(w, map[string]interface{}{
		"photos": photos,
	})
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

	mysql := db.GetMySQL()

	// 如果没有place_id，自动创建或查找
	placeID := req.PlaceID
	if placeID == 0 {
		delta := 0.0005 // ~50m
		var existID uint64
		mysql.QueryRowContext(r.Context(),
			`SELECT id FROM places WHERE status = 1
			AND latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ? LIMIT 1`,
			req.Latitude-delta, req.Latitude+delta, req.Longitude-delta, req.Longitude+delta,
		).Scan(&existID)

		if existID > 0 {
			placeID = existID
		} else {
			name := req.PlaceName
			if name == "" {
				name = "记忆点"
			}
			result, err := mysql.ExecContext(r.Context(),
				`INSERT INTO places (name, latitude, longitude, creator_id) VALUES (?, ?, ?, ?)`,
				name, req.Latitude, req.Longitude, userID,
			)
			if err != nil {
				util.Error(w, 500, "创建地点失败")
				return nil
			}
			id, _ := result.LastInsertId()
			placeID = uint64(id)
		}
	}

	// 验证地理位置（50米范围）
	var placeLat, placeLng float64
	mysql.QueryRowContext(r.Context(),
		"SELECT latitude, longitude FROM places WHERE id = ?", placeID,
	).Scan(&placeLat, &placeLng)

	dist := util.CalcDistance(req.Latitude, req.Longitude, placeLat, placeLng)
	if dist > 200 { // 放宽到200米，考虑GPS误差
		util.Error(w, 403, "你不在记忆点附近，无法投递")
		return nil
	}

	// 批量插入照片
	var photoIDs []int64
	for i, imgURL := range req.ImageURLs {
		isPreview := 0
		if i < 3 {
			isPreview = 1 // 前3张设为可预览
		}
		result, err := mysql.ExecContext(r.Context(),
			`INSERT INTO photos (user_id, place_id, image_url, thumbnail_url, description, latitude, longitude, is_preview)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, placeID, imgURL, imgURL, req.Description, req.Latitude, req.Longitude, isPreview,
		)
		if err != nil {
			continue
		}
		id, _ := result.LastInsertId()
		photoIDs = append(photoIDs, id)
	}

	// 更新地点照片数
	mysql.ExecContext(r.Context(),
		"UPDATE places SET photo_count = (SELECT COUNT(*) FROM photos WHERE place_id = ? AND status = 1) WHERE id = ?",
		placeID, placeID,
	)

	// 记录足迹
	mysql.ExecContext(r.Context(),
		`INSERT INTO footprints (user_id, place_id) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE visit_count = visit_count + 1, last_visit_at = NOW()`,
		userID, placeID,
	)

	// 检查成就
	go checkAchievements(userID)

	util.Success(w, map[string]interface{}{
		"photo_ids": photoIDs,
		"place_id":  placeID,
	})
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

	mysql := db.GetMySQL()
	var photo model.Photo
	err := mysql.QueryRowContext(r.Context(),
		`SELECT p.id, p.user_id, p.place_id, p.image_url, p.thumbnail_url, p.description,
			p.latitude, p.longitude, p.like_count, p.comment_count, p.view_count,
			p.created_at, u.nickname, u.avatar_url, pl.name
		FROM photos p
		LEFT JOIN users u ON u.id = p.user_id
		LEFT JOIN places pl ON pl.id = p.place_id
		WHERE p.id = ? AND p.status = 1`, photoID,
	).Scan(&photo.ID, &photo.UserID, &photo.PlaceID, &photo.ImageURL, &photo.ThumbnailURL,
		&photo.Description, &photo.Latitude, &photo.Longitude,
		&photo.LikeCount, &photo.CommentCount, &photo.ViewCount,
		&photo.CreatedAt, &photo.UserName, &photo.UserAvatar, &photo.PlaceName)
	if err != nil {
		util.Error(w, 404, "照片不存在")
		return nil
	}

	// 增加浏览数
	mysql.ExecContext(r.Context(), "UPDATE photos SET view_count = view_count + 1 WHERE id = ?", photoID)

	// 检查点赞状态
	userID := middleware.GetUserID(r.Context())
	if userID > 0 {
		var likeID uint64
		mysql.QueryRowContext(r.Context(),
			"SELECT id FROM likes WHERE user_id = ? AND photo_id = ?", userID, photoID,
		).Scan(&likeID)
		photo.IsLiked = likeID > 0
	}

	// 获取同地点照片列表
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT id, image_url FROM photos WHERE place_id = ? AND status = 1 ORDER BY created_at DESC LIMIT 20`,
		photo.PlaceID,
	)
	var images []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id uint64
			var url string
			rows.Scan(&id, &url)
			images = append(images, map[string]interface{}{"id": id, "image_url": url})
		}
	}

	result := map[string]interface{}{
		"id":            photo.ID,
		"user_id":       photo.UserID,
		"place_id":      photo.PlaceID,
		"image_url":     photo.ImageURL,
		"thumbnail_url": photo.ThumbnailURL,
		"description":   photo.Description,
		"latitude":      photo.Latitude,
		"longitude":     photo.Longitude,
		"like_count":    photo.LikeCount,
		"comment_count": photo.CommentCount,
		"view_count":    photo.ViewCount,
		"created_at":    photo.CreatedAt,
		"user_name":     photo.UserName,
		"user_avatar":   photo.UserAvatar,
		"place_name":    photo.PlaceName,
		"is_liked":      photo.IsLiked,
		"images":        images,
	}

	util.Success(w, result)
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

	var req struct {
		Action string `json:"action"` // "like" or "unlike"
	}
	util.ParseJSON(r, &req)

	mysql := db.GetMySQL()

	if req.Action == "unlike" {
		mysql.ExecContext(r.Context(),
			"DELETE FROM likes WHERE user_id = ? AND photo_id = ?", userID, photoID,
		)
		mysql.ExecContext(r.Context(),
			"UPDATE photos SET like_count = GREATEST(like_count - 1, 0) WHERE id = ?", photoID,
		)
	} else {
		// 点赞
		_, err := mysql.ExecContext(r.Context(),
			"INSERT IGNORE INTO likes (user_id, photo_id) VALUES (?, ?)", userID, photoID,
		)
		if err == nil {
			mysql.ExecContext(r.Context(),
				"UPDATE photos SET like_count = like_count + 1 WHERE id = ?", photoID,
			)
		}
		// 更新地点总赞数
		var placeID uint64
		mysql.QueryRowContext(r.Context(),
			"SELECT place_id FROM photos WHERE id = ?", photoID,
		).Scan(&placeID)
		if placeID > 0 {
			mysql.ExecContext(r.Context(),
				`UPDATE places SET like_count = (SELECT COALESCE(SUM(like_count), 0) FROM photos WHERE place_id = ? AND status = 1) WHERE id = ?`,
				placeID, placeID,
			)
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

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT c.id, c.content, c.created_at, u.nickname, u.avatar_url
		FROM comments c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.photo_id = ? AND c.status = 1
		ORDER BY c.created_at DESC
		LIMIT 50`, photoID,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		rows.Scan(&c.ID, &c.Content, &c.CreatedAt, &c.UserName, &c.UserAvatar)
		comments = append(comments, c)
	}

	util.Success(w, map[string]interface{}{
		"comments": comments,
	})
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

	mysql := db.GetMySQL()

	// 验证照片是否存在
	var exists int
	mysql.QueryRowContext(r.Context(),
		"SELECT 1 FROM photos WHERE id = ? AND status = 1", photoID,
	).Scan(&exists)
	if exists == 0 {
		util.Error(w, 404, "照片不存在")
		return nil
	}

	result, err := mysql.ExecContext(r.Context(),
		"INSERT INTO comments (photo_id, user_id, content, latitude, longitude) VALUES (?, ?, ?, ?, ?)",
		photoID, userID, req.Content, req.Latitude, req.Longitude,
	)
	if err != nil {
		util.Error(w, 500, "发表评论失败")
		return nil
	}

	// 更新评论数
	mysql.ExecContext(r.Context(),
		"UPDATE photos SET comment_count = comment_count + 1 WHERE id = ?", photoID,
	)

	id, _ := result.LastInsertId()
	util.Success(w, map[string]interface{}{
		"id": id,
	})
	return nil
}

// FavoritePhoto 收藏/取消收藏照片
func FavoritePhoto(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	var req struct {
		PhotoID uint64 `json:"photo_id"`
		Action  string `json:"action"` // "add" or "remove"
	}
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}

	mysql := db.GetMySQL()
	if req.Action == "remove" {
		mysql.ExecContext(r.Context(),
			"DELETE FROM favorites WHERE user_id = ? AND photo_id = ?", userID, req.PhotoID,
		)
	} else {
		mysql.ExecContext(r.Context(),
			"INSERT IGNORE INTO favorites (user_id, photo_id) VALUES (?, ?)", userID, req.PhotoID,
		)
	}

	util.Success(w, nil)
	return nil
}

// GetUserPhotos 获取用户的照片列表
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

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT p.id, p.image_url, p.thumbnail_url, p.description, p.like_count, p.comment_count,
			p.created_at, pl.name as place_name
		FROM photos p
		LEFT JOIN places pl ON pl.id = p.place_id
		WHERE p.user_id = ? AND p.status = 1
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?`, userID, pageSize, offset,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var photos []map[string]interface{}
	for rows.Next() {
		var id uint64
		var imgURL, thumbURL string
		var desc sql.NullString
		var likeCount, commentCount int
		var createdAt string
		var placeName sql.NullString
		rows.Scan(&id, &imgURL, &thumbURL, &desc, &likeCount, &commentCount, &createdAt, &placeName)
		photos = append(photos, map[string]interface{}{
			"id":            id,
			"image_url":     imgURL,
			"thumbnail_url": thumbURL,
			"description":   desc.String,
			"like_count":    likeCount,
			"comment_count": commentCount,
			"created_at":    createdAt,
			"place_name":    placeName.String,
		})
	}

	util.Success(w, map[string]interface{}{
		"photos": photos,
	})
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

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT p.id, p.image_url, p.thumbnail_url, p.description, p.like_count,
			p.created_at, pl.name as place_name
		FROM favorites f
		JOIN photos p ON p.id = f.photo_id AND p.status = 1
		LEFT JOIN places pl ON pl.id = p.place_id
		WHERE f.user_id = ?
		ORDER BY f.created_at DESC
		LIMIT ? OFFSET ?`, userID, pageSize, offset,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var photos []map[string]interface{}
	for rows.Next() {
		var id uint64
		var imgURL, thumbURL string
		var desc sql.NullString
		var likeCount int
		var createdAt string
		var placeName sql.NullString
		rows.Scan(&id, &imgURL, &thumbURL, &desc, &likeCount, &createdAt, &placeName)
		photos = append(photos, map[string]interface{}{
			"id":            id,
			"image_url":     imgURL,
			"thumbnail_url": thumbURL,
			"description":   desc.String,
			"like_count":    likeCount,
			"created_at":    createdAt,
			"place_name":    placeName.String,
		})
	}

	util.Success(w, map[string]interface{}{
		"photos": photos,
	})
	return nil
}

// GetUserFootprints 获取用户足迹
func GetUserFootprints(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT p.id, p.name, p.latitude, p.longitude, p.city, p.photo_count,
			f.visit_count, f.first_visit_at, f.last_visit_at
		FROM footprints f
		JOIN places p ON p.id = f.place_id
		WHERE f.user_id = ?
		ORDER BY f.last_visit_at DESC`, userID,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var footprints []map[string]interface{}
	for rows.Next() {
		var id uint64
		var name string
		var lat, lng float64
		var city string
		var photoCount, visitCount int
		var firstVisit, lastVisit string
		rows.Scan(&id, &name, &lat, &lng, &city, &photoCount, &visitCount, &firstVisit, &lastVisit)
		footprints = append(footprints, map[string]interface{}{
			"id":             id,
			"name":           name,
			"latitude":       lat,
			"longitude":      lng,
			"city":           city,
			"photo_count":    photoCount,
			"visit_count":    visitCount,
			"first_visit_at": firstVisit,
			"last_visit_at":  lastVisit,
		})
	}

	util.Success(w, map[string]interface{}{
		"footprints": footprints,
	})
	return nil
}

// checkAchievements 检查并解锁成就（异步）
func checkAchievements(userID uint64) {
	mysql := db.GetMySQL()
	if mysql == nil {
		return
	}

	// 获取用户统计
	var photoCount, placeCount, likeReceived, commentCount int
	mysql.QueryRow("SELECT COUNT(*) FROM photos WHERE user_id = ? AND status = 1", userID).Scan(&photoCount)
	mysql.QueryRow("SELECT COUNT(*) FROM footprints WHERE user_id = ?", userID).Scan(&placeCount)
	mysql.QueryRow("SELECT COALESCE(SUM(like_count), 0) FROM photos WHERE user_id = ? AND status = 1", userID).Scan(&likeReceived)
	mysql.QueryRow("SELECT COUNT(*) FROM comments WHERE user_id = ? AND status = 1", userID).Scan(&commentCount)

	// 获取城市数
	var cityCount int
	mysql.QueryRow(
		`SELECT COUNT(DISTINCT p.city) FROM footprints f
		JOIN places p ON p.id = f.place_id WHERE f.user_id = ?`, userID,
	).Scan(&cityCount)

	// 检查每个成就
	rows, _ := mysql.Query(
		`SELECT ad.id, ad.condition_type, ad.condition_value, ad.exp_reward
		FROM achievement_defs ad
		WHERE ad.status = 1 AND ad.id NOT IN (SELECT achievement_id FROM user_achievements WHERE user_id = ?)`, userID,
	)
	if rows == nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var condType string
		var condValue, expReward int
		rows.Scan(&id, &condType, &condValue, &expReward)

		var current int
		switch condType {
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

		if current >= condValue {
			mysql.Exec("INSERT IGNORE INTO user_achievements (user_id, achievement_id) VALUES (?, ?)", userID, id)
			mysql.Exec("UPDATE users SET exp = exp + ?, updated_at = NOW() WHERE id = ?", expReward, userID)

			// 检查升级
			var totalExp int
			mysql.QueryRow("SELECT exp FROM users WHERE id = ?", userID).Scan(&totalExp)
			newLevel := 1
			if totalExp >= 200 { newLevel = 6 } else if totalExp >= 150 { newLevel = 5 } else if totalExp >= 100 { newLevel = 4 } else if totalExp >= 50 { newLevel = 3 } else if totalExp >= 20 { newLevel = 2 }
			mysql.Exec("UPDATE users SET level = ?, updated_at = NOW() WHERE id = ? AND level < ?", newLevel, userID, newLevel)
		}
	}
}
