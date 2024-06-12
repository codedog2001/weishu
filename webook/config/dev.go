//go:build !k8s

package config

var Config = config{
	DB: DBConfig{
		DSN: "root:root@tcp(webook-live-mysql:13316)/webook",
	},
	Redis: RedisConfig{
		Add: "localhost:6379",
	},
}
