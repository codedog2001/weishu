//go:build wireinject

package main

import (
	"github.com/google/wire"
	"xiaoweishu/webook/interactive/events"
	repository2 "xiaoweishu/webook/interactive/repository"
	cache2 "xiaoweishu/webook/interactive/repository/cache"
	dao2 "xiaoweishu/webook/interactive/repository/dao"
	service2 "xiaoweishu/webook/interactive/service"
	"xiaoweishu/webook/internal/events/article"
	"xiaoweishu/webook/internal/repository"
	"xiaoweishu/webook/internal/repository/cache"
	"xiaoweishu/webook/internal/repository/dao"
	"xiaoweishu/webook/internal/service"
	"xiaoweishu/webook/internal/web"
	ijwt "xiaoweishu/webook/internal/web/jwt"
	"xiaoweishu/webook/ioc"
)

var interactiveSvcSet = wire.NewSet(dao2.NewGORMInteractiveDAO,
	cache2.NewInteractiveRedisCache,
	repository2.NewCachedInteractiveRepository,
	service2.NewInteractiveService,
)

var rankingSvcSet = wire.NewSet(
	cache.NewRankingRedisCache,
	repository.NewCachedRankingRepository,
	service.NewBatchRankingService,
)

func InitWebServer() *App {
	wire.Build(
		// 第三方依赖
		ioc.InitRedis, ioc.InitDB,
		ioc.InitLogger,
		ioc.InitEtcd,
		ioc.InitSaramaClient,
		ioc.InitSyncProducer,
		ioc.InitRlockClient,
		// DAO 部分
		dao.NewUserDAO,
		dao.NewArticleGORMDAO,

		interactiveSvcSet,
		ioc.InitIntrClientV1,
		rankingSvcSet,
		ioc.InitJobs,
		ioc.InitRankingJob,
		ioc.InitLikeJob,

		article.NewSaramaSyncProducer,
		events.NewInteractiveReadEventConsumer,
		ioc.InitConsumers,

		// cache 部分
		cache.NewCodeCache, cache.NewUserCache,
		cache.NewArticleRedisCache,

		// repository 部分
		repository.NewCacheUserRepository,
		repository.NewCodeRepository,
		repository.NewCachedArticleRepository,

		// Service 部分
		ioc.InitSMSService,
		ioc.InitWechatService,
		service.NewUserService,
		service.NewCodeService,
		service.NewArticleService,

		// handler 部分
		web.NewUserHandLer,
		web.NewArticleHandler,
		ijwt.NewRedisJWTHandler,
		web.NewOAuth2WechatHandler,
		ioc.InitGinMiddlewares,
		ioc.InitWebServer,

		wire.Struct(new(App), "*"),
	)
	return new(App)
}
