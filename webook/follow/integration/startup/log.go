package startup

import (
	"xiaoweishu/webook/pkg/logger"
)

func InitLog() logger.LoggerV1 {
	return logger.NewNopLogger()
}
