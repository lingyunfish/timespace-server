package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"timespace/config"
	"timespace/db"
	"timespace/middleware"
	"timespace/model"
	"timespace/util"

	trmysql "trpc.group/trpc-go/trpc-database/mysql"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Code string `json:"code"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string      `json:"token"`
	UserInfo *model.User `json:"user_info"`
}

// WxLoginResp 微信登录接口返回
type WxLoginResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// UserDB 用于 sqlx 映射的用户结构体
type UserDB struct {
	ID        uint64 `db:"id"`
	OpenID    string `db:"openid"`
	Nickname  string `db:"nickname"`
	AvatarURL string `db:"avatar_url"`
	Gender    int    `db:"gender"`
	Level     int    `db:"level"`
	Exp       int    `db:"exp"`
	IsVIP     int    `db:"is_vip"`
	Status    int    `db:"status"`
	CreatedAt string `db:"created_at"`
}

func toModelUser(u *UserDB) model.User {
	return model.User{
		ID:        u.ID,
		OpenID:    u.OpenID,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		Gender:    u.Gender,
		Level:     u.Level,
		Exp:       u.Exp,
		IsVIP:     u.IsVIP != 0,
		Status:    u.Status,
	}
}

// UserLogin 微信小程序登录
func UserLogin(w http.ResponseWriter, r *http.Request) error {
	var req LoginRequest
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}
	if req.Code == "" {
		util.Error(w, 400, "code不能为空")
		return nil
	}

	// 调用微信接口获取openid
	cfg := config.Get().WeChat
	wxURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		cfg.AppID, cfg.AppSecret, req.Code,
	)
	wxResp, err := http.Get(wxURL)
	if err != nil {
		util.Error(w, 500, "微信登录失败")
		return nil
	}
	defer wxResp.Body.Close()

	body, _ := io.ReadAll(wxResp.Body)
	var wxLogin WxLoginResp
	if err := json.Unmarshal(body, &wxLogin); err != nil || wxLogin.ErrCode != 0 {
		wxLogin.OpenID = "dev_" + req.Code
		wxLogin.SessionKey = "dev_session"
	}

	// 通过 tRPC mysql proxy 查询或创建用户
	proxy := db.GetMySQLProxy()
	ctx := r.Context()

	var userDB UserDB
	err = proxy.QueryToStruct(ctx, &userDB,
		"SELECT id, openid, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE openid = ?",
		wxLogin.OpenID,
	)

	var user model.User
	if err != nil {
		// 新用户注册
		result, err := proxy.Exec(ctx,
			"INSERT INTO users (openid, union_id, session_key, nickname, level) VALUES (?, ?, ?, ?, ?)",
			wxLogin.OpenID, wxLogin.UnionID, wxLogin.SessionKey, "时空旅行者", 1,
		)
		if err != nil {
			util.Error(w, 500, "创建用户失败")
			return nil
		}
		id, _ := result.LastInsertId()
		user = model.User{
			ID: uint64(id), OpenID: wxLogin.OpenID,
			Nickname: "时空旅行者", Level: 1, Status: 1,
		}
	} else {
		user = toModelUser(&userDB)
		proxy.Exec(ctx,
			"UPDATE users SET session_key = ?, updated_at = NOW() WHERE id = ?",
			wxLogin.SessionKey, user.ID,
		)
	}

	// 生成token
	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		util.Error(w, 500, "生成token失败")
		return nil
	}

	// 缓存
	rdb := db.GetRedis()
	userJSON, _ := json.Marshal(user)
	rdb.Set(ctx, fmt.Sprintf("user:%d", user.ID), userJSON, 24*time.Hour)

	util.Success(w, LoginResponse{Token: token, UserInfo: &user})
	return nil
}

// UpdateUserInfoRequest 更新用户信息请求
type UpdateUserInfoRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Gender    int    `json:"gender"`
}

// GetUserInfo 获取当前用户信息
func GetUserInfo(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	rdb := db.GetRedis()
	cached, err := rdb.Get(r.Context(), fmt.Sprintf("user:%d", userID)).Result()
	if err == nil {
		var user model.User
		if json.Unmarshal([]byte(cached), &user) == nil {
			util.Success(w, user)
			return nil
		}
	}

	proxy := db.GetMySQLProxy()
	var userDB UserDB
	err = proxy.QueryToStruct(r.Context(), &userDB,
		"SELECT id, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE id = ?",
		userID,
	)
	if err != nil {
		util.Error(w, 404, "用户不存在")
		return nil
	}
	user := toModelUser(&userDB)

	userJSON, _ := json.Marshal(user)
	rdb.Set(r.Context(), fmt.Sprintf("user:%d", userID), userJSON, 24*time.Hour)

	util.Success(w, user)
	return nil
}

// UpdateUserInfo 更新用户信息
func UpdateUserInfo(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}
	var req UpdateUserInfoRequest
	if err := util.ParseJSON(r, &req); err != nil {
		util.Error(w, 400, "参数错误")
		return nil
	}

	proxy := db.GetMySQLProxy()
	_, err := proxy.Exec(r.Context(),
		"UPDATE users SET nickname = ?, avatar_url = ?, gender = ?, updated_at = NOW() WHERE id = ?",
		req.Nickname, req.AvatarURL, req.Gender, userID,
	)
	if err != nil {
		util.Error(w, 500, "更新失败")
		return nil
	}

	rdb := db.GetRedis()
	rdb.Del(r.Context(), fmt.Sprintf("user:%d", userID))
	util.Success(w, nil)
	return nil
}

// GetUserStats 获取用户统计信息
func GetUserStats(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	proxy := db.GetMySQLProxy()
	ctx := r.Context()
	var stats model.UserStats

	proxy.QueryRow(ctx, []interface{}{&stats.PhotoCount},
		"SELECT COUNT(*) FROM photos WHERE user_id = ? AND status = 1", userID)
	proxy.QueryRow(ctx, []interface{}{&stats.PlaceCount},
		"SELECT COUNT(*) FROM footprints WHERE user_id = ?", userID)
	proxy.QueryRow(ctx, []interface{}{&stats.LikeReceived},
		"SELECT COALESCE(SUM(p.like_count), 0) FROM photos p WHERE p.user_id = ? AND p.status = 1", userID)
	proxy.QueryRow(ctx, []interface{}{&stats.AchievementCount},
		"SELECT COUNT(*) FROM user_achievements WHERE user_id = ?", userID)

	util.Success(w, stats)
	return nil
}

// AchievementDB 成就 sqlx 映射
type AchievementDB struct {
	ID             uint64 `db:"id"`
	Name           string `db:"name"`
	Description    string `db:"description"`
	Icon           string `db:"icon"`
	ConditionType  string `db:"condition_type"`
	ConditionValue int    `db:"condition_value"`
	ExpReward      int    `db:"exp_reward"`
	Unlocked       int    `db:"unlocked"`
}

// GetUserAchievements 获取用户成就列表
func GetUserAchievements(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	proxy := db.GetMySQLProxy()
	var rows []AchievementDB
	err := proxy.Select(r.Context(), &rows,
		`SELECT ad.id, ad.name, ad.description, ad.icon, ad.condition_type, ad.condition_value, ad.exp_reward,
			CASE WHEN ua.id IS NOT NULL THEN 1 ELSE 0 END as unlocked
		FROM achievement_defs ad
		LEFT JOIN user_achievements ua ON ua.achievement_id = ad.id AND ua.user_id = ?
		WHERE ad.status = 1
		ORDER BY ad.sort_order`, userID,
	)
	if err != nil {
		util.Error(w, 500, "查询失败")
		return nil
	}

	var achievements []model.Achievement
	for _, row := range rows {
		achievements = append(achievements, model.Achievement{
			AchievementDef: model.AchievementDef{
				ID: row.ID, Name: row.Name, Description: row.Description,
				Icon: row.Icon, ConditionType: row.ConditionType,
				ConditionValue: row.ConditionValue, ExpReward: row.ExpReward,
			},
			Unlocked: row.Unlocked == 1,
		})
	}

	util.Success(w, map[string]interface{}{"achievements": achievements})
	return nil
}

// 确保 trmysql 包被引用
var _ trmysql.Client
