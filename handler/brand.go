package handler

import (
	"net/http"
	"time"

	"timespace/db"
	"timespace/util"
)

// GetBrandMemories 获取地点的品牌记忆
func GetBrandMemories(w http.ResponseWriter, r *http.Request) error {
	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		util.Error(w, 400, "参数错误")
		return nil
	}

	mysql := db.GetMySQL()
	now := time.Now().Format("2006-01-02 15:04:05")
	rows, err := mysql.QueryContext(r.Context(),
		`SELECT id, brand_name, title, content, image_url, coupon_code
		FROM brand_memories
		WHERE place_id = ? AND status = 1 AND start_time <= ? AND end_time >= ?
		ORDER BY created_at DESC`,
		placeID, now, now,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}
	defer rows.Close()

	var memories []map[string]interface{}
	for rows.Next() {
		var id uint64
		var brandName, title, content, imageURL, couponCode string
		rows.Scan(&id, &brandName, &title, &content, &imageURL, &couponCode)
		memories = append(memories, map[string]interface{}{
			"id":          id,
			"brand_name":  brandName,
			"title":       title,
			"content":     content,
			"image_url":   imageURL,
			"coupon_code": couponCode,
		})
	}

	util.Success(w, map[string]interface{}{
		"memories": memories,
	})
	return nil
}
