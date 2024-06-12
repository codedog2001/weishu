package ioc

import (
	"xiaoweishu/webook/internal/service/oauth2/wechat"
	"xiaoweishu/webook/pkg/logger"
)

func InitWechatService(l logger.LoggerV1) wechat.Service {
	appID := "WECHAT_APP_ID"

	appSecret := "WECHAT_APP_SECRET"
	return wechat.NewService(appID, appSecret, l)
}
