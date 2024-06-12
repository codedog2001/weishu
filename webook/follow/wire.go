//go:build wireinject

package main

import (
	"github.com/google/wire"
	"xiaoweishu/webook/follow/grpc"
	"xiaoweishu/webook/follow/ioc"
	"xiaoweishu/webook/follow/repository"
	"xiaoweishu/webook/follow/repository/cache"
	"xiaoweishu/webook/follow/repository/dao"
	"xiaoweishu/webook/follow/service"
	ioc2 "xiaoweishu/webook/ioc"
)

var serviceProviderSet = wire.NewSet(
	cache.NewRedisFollowCache,
	dao.NewGORMFollowRelationDAO,
	repository.NewFollowRelationRepository,
	service.NewFollowRelationService,
	grpc.NewFollowRelationServiceServer,
	ioc2.InitRedis,
)

var thirdProvider = wire.NewSet(
	ioc.InitDB,
	ioc.InitLogger,
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
