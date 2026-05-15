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
	"trpc.group/trpc-go/trpc-go/log"
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
		ID: u.ID, OpenID: u.OpenID, Nickname: u.Nickname,
		AvatarURL: u.AvatarURL, Gender: u.Gender,
		Level: u.Level, Exp: u.Exp,
		IsVIP: u.IsVIP != 0, Status: u.Status,
	}
}

// UserLogin 微信小程序登录
func UserLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	var req LoginRequest
	if err := util.ParseJSON(r, &req); err != nil {
		util.ErrorCtx(ctx, w, 400, "参数错误", err)
		return nil
	}
	if req.Code == "" {
		util.ErrorCtx(ctx, w, 400, "code不能为空", nil)
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
		log.WithContext(ctx).Errorf("[WX LOGIN] call wechat api failed: %v, code=%s", err, req.Code)
		util.ErrorCtx(ctx, w, 500, "微信登录失败", err)
		return nil
	}
	defer wxResp.Body.Close()

	body, _ := io.ReadAll(wxResp.Body)
	var wxLogin WxLoginResp
	if err := json.Unmarshal(body, &wxLogin); err != nil || wxLogin.ErrCode != 0 {
		log.WithContext(ctx).Warnf("[WX LOGIN] wechat resp invalid, fallback to dev mode. resp=%s err=%v", string(body), err)
		wxLogin.OpenID = "dev_" + req.Code
		wxLogin.SessionKey = "dev_session"
	}

	proxy := db.GetMySQLProxy()
	if proxy == nil {
		util.ErrorCtx(ctx, w, 500, "数据库未连接", nil)
		return nil
	}

	var userDB UserDB
	queryErr := proxy.QueryToStruct(ctx, &userDB,
		"SELECT id, openid, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE openid = ?",
		wxLogin.OpenID,
	)

	var user model.User
	if queryErr != nil {
		// 新用户注册（QueryToStruct 找不到记录会返回 error，这里记录但继续插入）
		log.WithContext(ctx).Infof("[WX LOGIN] new user, openid=%s (query err: %v)", wxLogin.OpenID, queryErr)
		result, err := proxy.Exec(ctx,
			"INSERT INTO users (openid, union_id, session_key, nickname, level) VALUES (?, ?, ?, ?, ?)",
			wxLogin.OpenID, wxLogin.UnionID, wxLogin.SessionKey, "时空旅行者", 1,
		)
		if err != nil {
			log.WithContext(ctx).Errorf("[DB ERR] insert user failed, openid=%s err=%v", wxLogin.OpenID, err)
			util.ErrorCtx(ctx, w, 500, "创建用户失败", err)
			return nil
		}
		id, _ := result.LastInsertId()
		user = model.User{
			ID: uint64(id), OpenID: wxLogin.OpenID,
			Nickname: "时空旅行者", Level: 1, Status: 1,
		}
		log.WithContext(ctx).Infof("[WX LOGIN] user created, id=%d openid=%s", user.ID, user.OpenID)
	} else {
		user = toModelUser(&userDB)
		if _, err := proxy.Exec(ctx,
			"UPDATE users SET session_key = ?, updated_at = NOW() WHERE id = ?",
			wxLogin.SessionKey, user.ID,
		); err != nil {
			util.LogDBError(ctx, "update session_key", err, user.ID)
		}
	}

	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		log.WithContext(ctx).Errorf("[AUTH] generate token failed, uid=%d err=%v", user.ID, err)
		util.ErrorCtx(ctx, w, 500, "生成token失败", err)
		return nil
	}

	// 缓存
	if rdb := db.GetRedis(); rdb != nil {
		userJSON, _ := json.Marshal(user)
		if err := rdb.Set(ctx, fmt.Sprintf("user:%d", user.ID), userJSON, 24*time.Hour).Err(); err != nil {
			util.LogCacheError(ctx, "set user cache", err)
		}
	}

	log.WithContext(ctx).Infof("[WX LOGIN] success uid=%d openid=%s", user.ID, user.OpenID)
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
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		util.ErrorCtx(ctx, w, 401, "未登录", nil)
		return nil
	}

	rdb := db.GetRedis()
	if rdb != nil {
		cached, err := rdb.Get(ctx, fmt.Sprintf("user:%d", userID)).Result()
		if err == nil {
			var user model.User
			if json.Unmarshal([]byte(cached), &user) == nil {
				util.Success(w, user)
				return nil
			}
		}
	}

	proxy := db.GetMySQLProxy()
	var userDB UserDB
	err := proxy.QueryToStruct(ctx, &userDB,
		"SELECT id, nickname, avatar_url, gender, level, exp, is_vip, status, created_at FROM users WHERE id = ?",
		userID,
	)
	if err != nil {
		util.LogDBError(ctx, "query user info", err, userID)
		util.ErrorCtx(ctx, w, 404, "用户不存在", err)
		return nil
	}
	user := toModelUser(&userDB)

	if rdb != nil {
		userJSON, _ := json.Marshal(user)
		if err := rdb.Set(ctx, fmt.Sprintf("user:%d", userID), userJSON, 24*time.Hour).Err(); err != nil {
			util.LogCacheError(ctx, "set user cache", err)
		}
	}

	util.Success(w, user)
	return nil
}

// UpdateUserInfo 更新用户信息
func UpdateUserInfo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		util.ErrorCtx(ctx, w, 401, "未登录", nil)
		return nil
	}
	var req UpdateUserInfoRequest
	if err := util.ParseJSON(r, &req); err != nil {
		util.ErrorCtx(ctx, w, 400, "参数错误", err)
		return nil
	}

	proxy := db.GetMySQLProxy()
	_, err := proxy.Exec(ctx,
		"UPDATE users SET nickname = ?, avatar_url = ?, gender = ?, updated_at = NOW() WHERE id = ?",
		req.Nickname, req.AvatarURL, req.Gender, userID,
	)
	if err != nil {
		util.LogDBError(ctx, "update user info", err, userID)
		util.ErrorCtx(ctx, w, 500, "更新失败", err)
		return nil
	}

	if rdb := db.GetRedis(); rdb != nil {
		rdb.Del(ctx, fmt.Sprintf("user:%d", userID))
	}
	log.WithContext(ctx).Infof("[USER] update info success uid=%d", userID)
	util.Success(w, nil)
	return nil
}

// GetUserStats 获取用户统计信息
func GetUserStats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		util.ErrorCtx(ctx, w, 401, "未登录", nil)
		return nil
	}

	proxy := db.GetMySQLProxy()
	var stats model.UserStats

	if err := proxy.QueryRow(ctx, []interface{}{&stats.PhotoCount},
		"SELECT COUNT(*) FROM photos WHERE user_id = ? AND status = 1", userID); err != nil {
		util.LogDBError(ctx, "stats photo_count", err, userID)
	}
	if err := proxy.QueryRow(ctx, []interface{}{&stats.PlaceCount},
		"SELECT COUNT(*) FROM footprints WHERE user_id = ?", userID); err != nil {
		util.LogDBError(ctx, "stats place_count", err, userID)
	}
	if err := proxy.QueryRow(ctx, []interface{}{&stats.LikeReceived},
		"SELECT COALESCE(SUM(p.like_count), 0) FROM photos p WHERE p.user_id = ? AND p.status = 1", userID); err != nil {
		util.LogDBError(ctx, "stats like_received", err, userID)
	}
	if err := proxy.QueryRow(ctx, []interface{}{&stats.AchievementCount},
		"SELECT COUNT(*) FROM user_achievements WHERE user_id = ?", userID); err != nil {
		util.LogDBError(ctx, "stats achievement_count", err, userID)
	}

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
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		util.ErrorCtx(ctx, w, 401, "未登录", nil)
		return nil
	}

	proxy := db.GetMySQLProxy()
	var rows []AchievementDB
	err := proxy.Select(ctx, &rows,
		`SELECT ad.id, ad.name, ad.description, ad.icon, ad.condition_type, ad.condition_value, ad.exp_reward,
			CASE WHEN ua.id IS NOT NULL THEN 1 ELSE 0 END as unlocked
		FROM achievement_defs ad
		LEFT JOIN user_achievements ua ON ua.achievement_id = ad.id AND ua.user_id = ?
		WHERE ad.status = 1
		ORDER BY ad.sort_order`, userID,
	)
	if err != nil {
		util.LogDBError(ctx, "query achievements", err, userID)
		util.ErrorCtx(ctx, w, 500, "查询失败", err)
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
