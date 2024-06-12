package ioc

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"xiaoweishu/webook/pkg/logger"
)

func InitLogger() logger.LoggerV1 {
	// 直接使用 zap 本身的配置结构体来处理
	cfg := zap.NewDevelopmentConfig()
	err := viper.UnmarshalKey("log", &cfg)
	if err != nil {
		panic(err)
	}
	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger.NewZapLogger(l)
}
