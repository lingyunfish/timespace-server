package handler

import (
	"context"
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
		// 开发环境允许模拟登录
		wxLogin.OpenID = "dev_" + req.Code
		wxLogin.SessionKey = "dev_session"
	}

	// 查询或创建用户
	mysql := db.GetMySQL()
	var user model.User
	err = mysql.QueryRowContext(r.Context(),
		"SELECT id, openid, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE openid = ?",
		wxLogin.OpenID,
	).Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Gender,
		&user.Level, &user.Exp, &user.IsVIP, &user.Status, &user.CreatedAt)

	if err != nil {
		// 新用户注册
		result, err := mysql.ExecContext(r.Context(),
			"INSERT INTO users (openid, union_id, session_key, nickname, level) VALUES (?, ?, ?, ?, ?)",
			wxLogin.OpenID, wxLogin.UnionID, wxLogin.SessionKey, "时空旅行者", 1,
		)
		if err != nil {
			util.Error(w, 500, "创建用户失败")
			return nil
		}
		id, _ := result.LastInsertId()
		user = model.User{
			ID:       uint64(id),
			OpenID:   wxLogin.OpenID,
			Nickname: "时空旅行者",
			Level:    1,
			Status:   1,
		}
	} else {
		// 更新session_key
		mysql.ExecContext(r.Context(),
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

	// 缓存用户信息到Redis
	rdb := db.GetRedis()
	userJSON, _ := json.Marshal(user)
	rdb.Set(context.Background(), fmt.Sprintf("user:%d", user.ID), userJSON, 24*time.Hour)

	util.Success(w, LoginResponse{
		Token:    token,
		UserInfo: &user,
	})
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

	// 先查缓存
	rdb := db.GetRedis()
	cached, err := rdb.Get(r.Context(), fmt.Sprintf("user:%d", userID)).Result()
	if err == nil {
		var user model.User
		if json.Unmarshal([]byte(cached), &user) == nil {
			util.Success(w, user)
			return nil
		}
	}

	// 查数据库
	mysql := db.GetMySQL()
	var user model.User
	err = mysql.QueryRowContext(r.Context(),
		"SELECT id, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Nickname, &user.AvatarURL, &user.Gender,
		&user.Level, &user.Exp, &user.IsVIP, &user.Status, &user.CreatedAt)
	if err != nil {
		util.Error(w, 404, "用户不存在")
		return nil
	}

	// 更新缓存
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

	mysql := db.GetMySQL()
	_, err := mysql.ExecContext(r.Context(),
		"UPDATE users SET nickname = ?, avatar_url = ?, gender = ?, updated_at = NOW() WHERE id = ?",
		req.Nickname, req.AvatarURL, req.Gender, userID,
	)
	if err != nil {
		util.Error(w, 500, "更新失败")
		return nil
	}

	// 删除缓存
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

	mysql := db.GetMySQL()
	var stats model.UserStats

	mysql.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM photos WHERE user_id = ? AND status = 1", userID,
	).Scan(&stats.PhotoCount)

	mysql.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM footprints WHERE user_id = ?", userID,
	).Scan(&stats.PlaceCount)

	mysql.QueryRowContext(r.Context(),
		"SELECT COALESCE(SUM(p.like_count), 0) FROM photos p WHERE p.user_id = ? AND p.status = 1", userID,
	).Scan(&stats.LikeReceived)

	mysql.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM user_achievements WHERE user_id = ?", userID,
	).Scan(&stats.AchievementCount)

	util.Success(w, stats)
	return nil
}

// GetUserAchievements 获取用户成就列表
func GetUserAchievements(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		util.Error(w, 401, "未登录")
		return nil
	}

	mysql := db.GetMySQL()
	rows, err := mysql.QueryContext(r.Context(),
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
	defer rows.Close()

	var achievements []model.Achievement
	for rows.Next() {
		var a model.Achievement
		var unlocked int
		rows.Scan(&a.ID, &a.Name, &a.Description, &a.Icon, &a.ConditionType,
			&a.ConditionValue, &a.ExpReward, &unlocked)
		a.Unlocked = unlocked == 1
		achievements = append(achievements, a)
	}

	util.Success(w, map[string]interface{}{
		"achievements": achievements,
	})
	return nil
}
