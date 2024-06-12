//go:build wireinject

package startup

import (
	"github.com/google/wire"
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
)

var thirdPartySet = wire.NewSet( // 第三方依赖
	InitRedis, InitDB,
	InitLogger,
	InitSaramaClient,
	InitSyncProducer)

var userSvcProvider = wire.NewSet(
	dao.NewUserDAO,
	cache.NewUserCache,
	repository.NewUserRepository,
	service.NewUserService)

//
//func InitWebServer() *gin.Engine {
//	wire.Build(
//		thirdPartySet,
//		userSvcProvider,
//		articlSvcProvider,
//		// cache 部分
//		cache.NewCodeCache,
//
//		// repository 部分
//		repository.NewCodeRepository,
//
//		// Service 部分
//		ioc.InitSMSService,
//		service.NewCodeService,
//		InitWechatService,
//
//		// handler 部分
//		web.NewUserHandLer,
//		web.NewArticleHandler,
//		web.NewOAuth2WechatHandler,
//		ijwt.NewRedisJWTHandler,
//		ioc.InitGinMiddlewares,
//		ioc.InitWebServer,
//	)
//	return gin.Default()
//}

//var articlSvcProvider = wire.NewSet(
//	repository.NewCachedArticleRepository,
//	cache.NewArticleRedisCache,
//	dao.NewArticleGORMDAO,
//	service.NewArticleService)

var interactiveSvcSet = wire.NewSet(dao2.NewGORMInteractiveDAO,
	cache2.NewInteractiveRedisCache,
	repository2.NewCachedInteractiveRepository,
	service2.NewInteractiveService,
)

func InitArticleHandler(dao dao.ArticleDAO) *web.ArticleHandler {
	wire.Build(
		thirdPartySet,
		userSvcProvider,
		interactiveSvcSet,
		repository.NewCachedArticleRepository,
		cache.NewArticleRedisCache,
		article.NewSaramaSyncProducer,
		service.NewArticleService,
		web.NewArticleHandler)
	return &web.ArticleHandler{}
}
