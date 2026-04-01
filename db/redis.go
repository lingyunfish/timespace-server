package db

import (
	"context"

	"github.com/redis/go-redis/v9"
	"timespace/config"
)

var rdb *redis.Client

func InitRedis() error {
	cfg := config.Get().Redis
	rdb = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	return rdb.Ping(context.Background()).Err()
}

func GetRedis() *redis.Client {
	return rdb
}

func CloseRedis() {
	if rdb != nil {
		rdb.Close()
	}
}
