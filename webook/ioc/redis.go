package ioc

import (
	rlock "github.com/gotomicro/redis-lock"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func InitRedis() redis.Cmdable {
	return redis.NewClient(&redis.Options{
		Addr: viper.GetString("redis.addr"),
	})
}

// 初始化基于redis实现的分布式锁
func InitRlockClient(client redis.Cmdable) *rlock.Client {
	return rlock.NewClient(client)
}
