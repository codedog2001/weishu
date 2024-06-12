package startup

import (
	"xiaoweishu/webook/internal/service/oauth2/wechat"
	"xiaoweishu/webook/pkg/logger"
)

func InitWechatService(l logger.LoggerV1) wechat.Service {
	return wechat.NewService("", "", l)
}
