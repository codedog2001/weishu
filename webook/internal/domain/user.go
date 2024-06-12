package domain

import "time"

// User 领域对象
type User struct {
	Id       int64
	Email    string
	Password string
	Ctime    time.Time
	Phone    string
	//不要直接进行组合，万一以后需要扩展其他平台，其他平台中也有同名字段就比较麻烦
	WechatInfo WechatInfo
	Nickname   string //用户昵称
}
