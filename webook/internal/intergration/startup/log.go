package startup

import (
	logger2 "xiaoweishu/webook/pkg/logger"
)

func InitLogger() logger2.LoggerV1 {
	return logger2.NewNopLogger() //也就是暂时先不用日志，测试用不到日志
}
