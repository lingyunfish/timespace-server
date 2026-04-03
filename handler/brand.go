package handler

import (
	"net/http"
	"time"

	"timespace/db"
	"timespace/util"
)

type BrandMemoryDB struct {
	ID         uint64 `db:"id"`
	BrandName  string `db:"brand_name"`
	Title      string `db:"title"`
	Content    string `db:"content"`
	ImageURL   string `db:"image_url"`
	CouponCode string `db:"coupon_code"`
}

// GetBrandMemories 获取地点的品牌记忆
func GetBrandMemories(w http.ResponseWriter, r *http.Request) error {
	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		util.Error(w, 400, "参数错误")
		return nil
	}

	proxy := db.GetMySQLProxy()
	now := time.Now().Format("2006-01-02 15:04:05")

	var rows []BrandMemoryDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT id, brand_name, title, COALESCE(content,'') as content, COALESCE(image_url,'') as image_url, COALESCE(coupon_code,'') as coupon_code
		FROM brand_memories
		WHERE place_id = ? AND status = 1 AND start_time <= ? AND end_time >= ?
		ORDER BY created_at DESC`, placeID, now, now)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}

	util.Success(w, map[string]interface{}{"memories": rows})
	return nil
}
