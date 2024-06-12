package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"xiaoweishu/webook/interactive/events"
	"xiaoweishu/webook/interactive/grpc"
	"xiaoweishu/webook/interactive/ioc"
	"xiaoweishu/webook/interactive/repository"
	"xiaoweishu/webook/interactive/repository/cache"
	"xiaoweishu/webook/interactive/repository/dao"
	"xiaoweishu/webook/interactive/service"
	events2 "xiaoweishu/webook/internal/events"
	ioc2 "xiaoweishu/webook/ioc"
	"xiaoweishu/webook/pkg/ginx"
	"xiaoweishu/webook/pkg/grpcx"
)

type App struct {
	consumers   []events2.Consumer
	server      *grpcx.Server
	adminServer *ginx.Server
}

func InitApp() *App {
	srcDB := ioc.InitSrcDB()
	dstDB := ioc.InitDstDB()
	loggerV1 := ioc.InitLogger()
	doubleWritePool := ioc.InitDoubleWritePool(srcDB, dstDB, loggerV1)
	db := ioc.InitBizDB(doubleWritePool)
	interactiveDAO := dao.NewGORMInteractiveDAO(db)
	cmdable := ioc.InitRedis()
	interactiveCache := cache.NewInteractiveRedisCache(cmdable)
	interactiveRepository := repository.NewCachedInteractiveRepository(interactiveDAO, interactiveCache, loggerV1)
	client := ioc.InitSaramaClient()
	interactiveReadEventConsumer := events.NewInteractiveReadEventConsumer(interactiveRepository, client, loggerV1)
	consumer := ioc.InitFixerConsumer(client, loggerV1, srcDB, dstDB)
	v := ioc.InitConsumers(interactiveReadEventConsumer, consumer)
	interactiveService := service.NewInteractiveService(interactiveRepository)
	interactiveServiceServer := grpc.NewInteractiveServiceServer(interactiveService)
	clientv3Client := ioc2.InitEtcd()
	server := ioc.NewGrpcxServer(interactiveServiceServer, clientv3Client, loggerV1)
	syncProducer := ioc.InitSaramaSyncProducer(client)
	producer := ioc.InitInteractiveProducer(syncProducer)
	ginxServer := ioc.InitGinxSever(loggerV1, srcDB, dstDB, doubleWritePool, producer)
	app := &App{
		consumers:   v,
		server:      server,
		adminServer: ginxServer,
	}
	return app
}

func main() {
	initViperV1()
	app := InitApp()
	initPrometheus()
	for _, consumer := range app.consumers {
		err := consumer.Start()
		if err != nil {
			panic(err)
		}
	}
	go func() {
		err1 := app.adminServer.Start()
		panic(err1)
	}()
	err := app.server.Serve()
	if err != nil {
		panic(err)
	}
}
func initPrometheus() {
	go func() {
		// 专门给 prometheus 用的端口
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":8081", nil)
		if err != nil {
			return
		}
	}()
}
func initViperV1() {
	cfile := pflag.String("config",
		"config/config.yaml", "配置文件路径")
	// 这一步之后，cfile 里面才有值
	pflag.Parse()
	//viper.Set("db.dsn", "localhost:3306")
	// 所有的默认值放好s
	viper.SetConfigType("yaml")
	viper.SetConfigFile(*cfile)
	// 读取配置
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	val := viper.Get("test.key")
	log.Println(val)
}
