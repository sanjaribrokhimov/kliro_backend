package utils

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var redisClient *redis.Client

func SetRedis(client *redis.Client) {
	redisClient = client
}

func GetRedis() *redis.Client {
	return redisClient
}

var ctx = context.Background()

func RedisCtx() context.Context {
	return ctx
}
