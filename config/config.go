package config

import (
	"encoding/json"
	"os"
	"sync"
)

var (
	conf *Config
	once sync.Once
)

type Config struct {
	Server  ServerConfig  `json:"server"`
	MySQL   MySQLConfig   `json:"mysql"`
	Redis   RedisConfig   `json:"redis"`
	WeChat  WeChatConfig  `json:"wechat"`
	JWT     JWTConfig     `json:"jwt"`
	Upload  UploadConfig  `json:"upload"`
	Geo     GeoConfig     `json:"geo"`
}

type ServerConfig struct {
	Port int `json:"port"`
}

type MySQLConfig struct {
	DSN             string `json:"dsn"`
	MaxOpenConns    int    `json:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	ConnMaxLifetime int    `json:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

type WeChatConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

type JWTConfig struct {
	Secret      string `json:"secret"`
	ExpireHours int    `json:"expire_hours"`
}

type UploadConfig struct {
	MaxSize      int64    `json:"max_size"`
	AllowedTypes []string `json:"allowed_types"`
	SavePath     string   `json:"save_path"`
	URLPrefix    string   `json:"url_prefix"`
}

type GeoConfig struct {
	NearbyRadius float64 `json:"nearby_radius"`
	SearchRadius float64 `json:"search_radius"`
}

func Load(path string) (*Config, error) {
	var err error
	once.Do(func() {
		var data []byte
		data, err = os.ReadFile(path)
		if err != nil {
			return
		}
		conf = &Config{}
		err = json.Unmarshal(data, conf)
	})
	return conf, err
}

func Get() *Config {
	return conf
}
