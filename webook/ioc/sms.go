package ioc

import (
	"xiaoweishu/webook/internal/service/sms"
	"xiaoweishu/webook/internal/service/sms/memory"
)

func InitSMSService() sms.Service {
	return memory.NewService()
	// 如果有需要，就可以用这个
	//return initTencentSMSService()
}
