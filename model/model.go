package model

import "time"

type User struct {
	ID         uint64     `json:"id"`
	OpenID     string     `json:"openid,omitempty"`
	UnionID    string     `json:"union_id,omitempty"`
	SessionKey string     `json:"-"`
	Nickname   string     `json:"nickname"`
	AvatarURL  string     `json:"avatar_url"`
	Gender     int        `json:"gender"`
	Level      int        `json:"level"`
	Exp        int        `json:"exp"`
	IsVIP      bool       `json:"is_vip"`
	VIPExpireAt *time.Time `json:"vip_expire_at,omitempty"`
	Status     int        `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Place struct {
	ID           uint64    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Address      string    `json:"address"`
	City         string    `json:"city"`
	Province     string    `json:"province"`
	Country      string    `json:"country"`
	CoverURL     string    `json:"cover_url"`
	PhotoCount   int       `json:"photo_count"`
	VisitorCount int       `json:"visitor_count"`
	LikeCount    int       `json:"like_count"`
	IsOfficial   bool      `json:"is_official"`
	Category     string    `json:"category"`
	CreatorID    uint64    `json:"creator_id"`
	Status       int       `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// 非数据库字段
	Distance     float64  `json:"distance,omitempty"`
	DistanceText string   `json:"distance_text,omitempty"`
	Photos       []Photo  `json:"photos,omitempty"`
}

type Photo struct {
	ID           uint64    `json:"id"`
	UserID       uint64    `json:"user_id"`
	PlaceID      uint64    `json:"place_id"`
	ImageURL     string    `json:"image_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Description  string    `json:"description"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	LikeCount    int       `json:"like_count"`
	CommentCount int       `json:"comment_count"`
	ViewCount    int       `json:"view_count"`
	IsPreview    bool      `json:"is_preview"`
	Status       int       `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// 非数据库字段
	UserName     string    `json:"user_name,omitempty"`
	UserAvatar   string    `json:"user_avatar,omitempty"`
	PlaceName    string    `json:"place_name,omitempty"`
	IsLiked      bool      `json:"is_liked,omitempty"`
}

type Comment struct {
	ID        uint64    `json:"id"`
	PhotoID   uint64    `json:"photo_id"`
	UserID    uint64    `json:"user_id"`
	Content   string    `json:"content"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	ReplyTo   uint64    `json:"reply_to"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// 非数据库字段
	UserName   string   `json:"user_name,omitempty"`
	UserAvatar string   `json:"user_avatar,omitempty"`
}

type Like struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	PhotoID   uint64    `json:"photo_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Footprint struct {
	ID           uint64    `json:"id"`
	UserID       uint64    `json:"user_id"`
	PlaceID      uint64    `json:"place_id"`
	VisitCount   int       `json:"visit_count"`
	FirstVisitAt time.Time `json:"first_visit_at"`
	LastVisitAt  time.Time `json:"last_visit_at"`
}

type AchievementDef struct {
	ID             uint64 `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Icon           string `json:"icon"`
	ConditionType  string `json:"condition_type"`
	ConditionValue int    `json:"condition_value"`
	ExpReward      int    `json:"exp_reward"`
	SortOrder      int    `json:"sort_order"`
}

type UserAchievement struct {
	ID            uint64    `json:"id"`
	UserID        uint64    `json:"user_id"`
	AchievementID uint64    `json:"achievement_id"`
	UnlockedAt    time.Time `json:"unlocked_at"`
}

type Achievement struct {
	AchievementDef
	Unlocked bool   `json:"unlocked"`
	Progress string `json:"progress,omitempty"`
}

type Favorite struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	PhotoID   uint64    `json:"photo_id"`
	CreatedAt time.Time `json:"created_at"`
}

type BrandMemory struct {
	ID         uint64    `json:"id"`
	BrandName  string    `json:"brand_name"`
	PlaceID    uint64    `json:"place_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	ImageURL   string    `json:"image_url"`
	CouponCode string    `json:"coupon_code"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserStats struct {
	PhotoCount       int `json:"photo_count"`
	PlaceCount       int `json:"place_count"`
	LikeReceived     int `json:"like_received"`
	AchievementCount int `json:"achievement_count"`
}
