//go:build wireinject

package main

import (
	"github.com/google/wire"
	"xiaoweishu/webook/interactive/events"
	"xiaoweishu/webook/interactive/grpc"
	"xiaoweishu/webook/interactive/ioc"
	repository2 "xiaoweishu/webook/interactive/repository"
	cache2 "xiaoweishu/webook/interactive/repository/cache"
	dao2 "xiaoweishu/webook/interactive/repository/dao"
	service2 "xiaoweishu/webook/interactive/service"
	ioc2 "xiaoweishu/webook/ioc"
)

var thirdPartySet = wire.NewSet(ioc.InitSrcDB,
	ioc.InitDstDB,
	ioc.InitDoubleWritePool,
	ioc.InitBizDB,
	ioc.InitLogger,
	ioc.InitSaramaClient,
	ioc.InitSaramaSyncProducer,
	ioc.InitRedis)

var interactiveSvcSet = wire.NewSet(dao2.NewGORMInteractiveDAO,
	cache2.NewInteractiveRedisCache,
	repository2.NewCachedInteractiveRepository,
	service2.NewInteractiveService,
)

func InitApp() *App {
	wire.Build(thirdPartySet,
		interactiveSvcSet,
		grpc.NewInteractiveServiceServer,
		events.NewInteractiveReadEventConsumer,
		ioc.InitInteractiveProducer,
		ioc.InitFixerConsumer,
		ioc.InitConsumers,
		ioc.NewGrpcxServer,
		ioc.InitGinxSever,
		ioc2.InitEtcd,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}
