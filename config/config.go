package config

import (
	"sync"

	"trpc.group/trpc-go/trpc-go/plugin"
)

const (
	pluginType = "custom"
	pluginName = "timespace"
)

var (
	conf *Config
	mu   sync.RWMutex
)

// Config 业务自定义配置（数据库配置由 trpc_go.yaml 的 client.service 管理）
type Config struct {
	WeChat WeChatConfig `yaml:"wechat"`
	JWT    JWTConfig    `yaml:"jwt"`
	Upload UploadConfig `yaml:"upload"`
	Geo    GeoConfig    `yaml:"geo"`
}

type WeChatConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type UploadConfig struct {
	MaxSize      int64    `yaml:"max_size"`
	AllowedTypes []string `yaml:"allowed_types"`
	SavePath     string   `yaml:"save_path"`
	URLPrefix    string   `yaml:"url_prefix"`
}

type GeoConfig struct {
	NearbyRadius float64 `yaml:"nearby_radius"`
	SearchRadius float64 `yaml:"search_radius"`
}

// Plugin 实现 trpc plugin.Factory 接口
type Plugin struct{}

func (p *Plugin) Type() string {
	return pluginType
}

func (p *Plugin) Setup(name string, decoder plugin.Decoder) error {
	cfg := &Config{}
	if err := decoder.Decode(cfg); err != nil {
		return err
	}
	mu.Lock()
	conf = cfg
	mu.Unlock()
	return nil
}

func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return conf
}

func init() {
	plugin.Register(pluginName, &Plugin{})
}
