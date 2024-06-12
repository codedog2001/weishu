//go:build wireinject

package main

import (
	"github.com/google/wire"
	"xiaoweishu/webook/comment/grpc"
	ioc "xiaoweishu/webook/comment/ioc"
	"xiaoweishu/webook/comment/repository"
	"xiaoweishu/webook/comment/repository/dao"
	"xiaoweishu/webook/comment/service"
	ioc2 "xiaoweishu/webook/ioc"
)

var serviceProviderSet = wire.NewSet(
	dao.NewCommentDAO,
	repository.NewCommentRepo,
	service.NewCommentSvc,
	grpc.NewGrpcServer,
)

var thirdProvider = wire.NewSet(
	ioc.InitLogger,
	ioc.InitDB,
	ioc2.InitEtcd,
)

func Init() *App {
	wire.Build(
		thirdProvider,
		serviceProviderSet,
		ioc.InitGRPCxServer,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}
