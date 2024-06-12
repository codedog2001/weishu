//go:build k8s

package config

var Config = config{
	DB: DBConfig{
		DSN: "root:root@tcp(webook-live-mysql:11309)/mysql",
	},
	Redis: RedisConfig{
		Add: "webook-live-redis:11479",
	},
}
