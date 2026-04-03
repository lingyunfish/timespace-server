package db

import (
	"strings"

	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
)

const redisServiceName = "trpc.redis.timespace.default"

var rdb *redis.Client

// InitRedis 从 trpc_go.yaml 的 client.service 中读取 Redis 地址并初始化连接
func InitRedis() error {
	cfg := trpc.GlobalConfig()
	addr := "127.0.0.1:6379"
	password := ""
	db := 0

	for _, svc := range cfg.Client.Service {
		if svc.ServiceName == redisServiceName {
			// target 格式: ip://host:port?db=0&password=xxx
			target := strings.TrimPrefix(svc.Target, "ip://")
			parts := strings.SplitN(target, "?", 2)
			addr = parts[0]
			if len(parts) > 1 {
				params := parseQueryParams(parts[1])
				if v, ok := params["password"]; ok {
					password = v
				}
				if v, ok := params["db"]; ok {
					if v == "1" {
						db = 1
					} // 简化处理，实际可用 strconv
				}
			}
			break
		}
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 100,
	})
	return rdb.Ping(trpc.BackgroundContext()).Err()
}

func GetRedis() *redis.Client {
	return rdb
}

func CloseRedis() {
	if rdb != nil {
		rdb.Close()
	}
}

// parseQueryParams 简单解析 key=value&key2=value2
func parseQueryParams(query string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(query, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
