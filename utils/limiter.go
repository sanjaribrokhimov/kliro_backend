package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func CanSendOTP(rdb *redis.Client, key string) (bool, string) {
	ctx := context.Background()
	minuteKey := fmt.Sprintf("otp_minute_%s", key)
	hourKey := fmt.Sprintf("otp_hour_%s", key)
	if rdb.Exists(ctx, minuteKey).Val() > 0 {
		return false, "Можно отправлять не чаще 1 раза в 60 секунд"
	}
	cnt, _ := rdb.Get(ctx, hourKey).Int()
	if cnt >= 10 {
		return false, "Можно отправлять не более 10 раз в час"
	}
	return true, ""
}

func MarkOTPSent(rdb *redis.Client, key string) {
	ctx := context.Background()
	minuteKey := fmt.Sprintf("otp_minute_%s", key)
	hourKey := fmt.Sprintf("otp_hour_%s", key)
	rdb.Set(ctx, minuteKey, 1, 60*time.Second)
	rdb.Incr(ctx, hourKey)
	rdb.Expire(ctx, hourKey, time.Hour)
}
