package main

import (
	"xiaoweishu/webook/internal/events"
	"xiaoweishu/webook/pkg/ginx"
	"xiaoweishu/webook/pkg/grpcx"
)

type App struct {
	consumers   []events.Consumer
	server      *grpcx.Server
	adminServer *ginx.Server
}
