package startup

//var thirdPartySet = wire.NewSet( // 第三方依赖
//	InitRedis, InitDB,
//	//InitSaramaClient,
//	//InitSyncProducer,
//	InitLogger,
//)
//
//var interactiveSvcSet = wire.NewSet(dao.NewGORMInteractiveDAO,
//	cache.NewInteractiveRedisCache,
//	repository.NewCachedInteractiveRepository,
//	service.NewInteractiveService,
//)
//
//func InitInteractiveService() *grpc.InteractiveServiceServer {
//	wire.Build(thirdPartySet, interactiveSvcSet, grpc.NewInteractiveServiceServer)
//	return new(grpc.InteractiveServiceServer)
//}
