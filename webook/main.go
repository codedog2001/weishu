package main

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"net/http"
	"time"
	events2 "xiaoweishu/webook/interactive/events"
	repository2 "xiaoweishu/webook/interactive/repository"
	cache2 "xiaoweishu/webook/interactive/repository/cache"
	dao2 "xiaoweishu/webook/interactive/repository/dao"
	"xiaoweishu/webook/internal/events"
	"xiaoweishu/webook/internal/events/article"
	"xiaoweishu/webook/internal/repository"
	"xiaoweishu/webook/internal/repository/cache"
	"xiaoweishu/webook/internal/repository/dao"
	"xiaoweishu/webook/internal/service"
	"xiaoweishu/webook/internal/web"
	"xiaoweishu/webook/internal/web/jwt"
	"xiaoweishu/webook/ioc"
)

func main() {
	initLogger()
	initViper()
	tpCancel := ioc.InitOTEL()
	defer func() {
		//新建一个带超时控制的ctx来取消tp
		//不过就算是关闭失败，超时了也会取消ctx
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		tpCancel(ctx)
	}()
	app := InitWebServerv1()
	initPrometheus()
	//消费者进程是一开始就要部署好，所以这里要启动消费者进程
	//直到生产者产生消息放入分区之后，消费者拿到消息之后，才会进行消费，消费者进程才会被激活
	for _, c := range app.consumers {
		err := c.Start()
		if err != nil {
			panic(err)
		}
	}
	server := app.server
	server.GET("/hello", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "hello，启动成功了！")
	})
	err := server.Run(":8080")
	if err != nil {
		return
	}
}

func initPrometheus() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":8081", nil)
		if err != nil {
			panic(err)
		}
	}()
}

type App struct {
	server    *gin.Engine
	consumers []events.Consumer
	cron      *cron.Cron
}

func InitWebServerv1() *App {
	cmdable := ioc.InitRedis()
	handler := jwt.NewRedisJWTHandler(cmdable)
	loggerV1 := ioc.InitLogger()
	v := ioc.InitGinMiddlewares(cmdable, handler, loggerV1)
	db := ioc.InitDB(loggerV1)
	userDAO := dao.NewUserDAO(db)
	userCache := cache.NewUserCache(cmdable)
	userRepository := repository.NewCacheUserRepository(userDAO, userCache)
	userService := service.NewUserService(userRepository)
	codeCache := cache.NewCodeCache(cmdable)
	codeRepository := repository.NewCodeRepository(codeCache)
	smsService := ioc.InitSMSService()
	codeSerVice := service.NewCodeService(codeRepository, smsService)
	userHandLer := web.NewUserHandLer(userService, codeSerVice, handler)
	wechatService := ioc.InitWechatService(loggerV1)
	oAuth2WechatHandLer := web.NewOAuth2WechatHandler(wechatService, userService, handler)
	articleDAO := dao.NewArticleGORMDAO(db)
	articleCache := cache.NewArticleRedisCache(cmdable)
	articleRepository := repository.NewCachedArticleRepository(articleDAO, userRepository, articleCache)
	client := ioc.InitSaramaClient()
	syncProducer := ioc.InitSyncProducer(client)
	producer := article.NewSaramaSyncProducer(syncProducer)
	articleService := service.NewArticleService(articleRepository, producer, loggerV1)
	clientv3Client := ioc.InitEtcd()
	interactiveServiceClient := ioc.InitIntrClientV1(clientv3Client)
	articleHandler := web.NewArticleHandler(loggerV1, articleService, interactiveServiceClient)
	engine := ioc.InitWebServer(v, userHandLer, oAuth2WechatHandLer, articleHandler)
	interactiveDAO := dao2.NewGORMInteractiveDAO(db)
	interactiveCache := cache2.NewInteractiveRedisCache(cmdable)
	interactiveRepository := repository2.NewCachedInteractiveRepository(interactiveDAO, interactiveCache, loggerV1)
	interactiveReadEventConsumer := events2.NewInteractiveReadEventConsumer(interactiveRepository, client, loggerV1)
	v2 := ioc.InitConsumers(interactiveReadEventConsumer)
	rankingService := service.NewBatchRankingService(interactiveServiceClient, articleService)
	rlockClient := ioc.InitRlockClient(cmdable)
	rankingJob := ioc.InitRankingJob(rankingService, rlockClient, loggerV1)
	updateLikeJob := ioc.InitLikeJob(articleService, rlockClient, loggerV1)
	cron := ioc.InitJobs(loggerV1, rankingJob, updateLikeJob)
	app := &App{
		server:    engine,
		consumers: v2,
		cron:      cron,
	}
	return app
}

func initLogger() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}
func initViper() {
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	// 当前工作目录的 config 子目录
	viper.AddConfigPath("config")
	// 读取配置
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	val := viper.Get("test.key")
	log.Println(val)
}

// 用viper和远程控制中心etcd配合使用
func initViperRemote() {
	err := viper.AddRemoteProvider("etcd3",
		"http://127.0.0.1:12379", "/webook")
	if err != nil {
		panic(err)
	}
	viper.SetConfigType("yaml")
	viper.OnConfigChange(func(in fsnotify.Event) {
		log.Println("远程配置中心发生变更")
	})
	go func() {
		for {
			err = viper.WatchRemoteConfig()
			if err != nil {
				panic(err)
			}
			log.Println("watch", viper.GetString("test.key"))
			//time.Sleep(time.Second)
		}
	}()
	err = viper.ReadRemoteConfig()
	if err != nil {
		panic(err)
	}
}

//func useSession(server *gin.Engine) {
//	login := &middleware.LoginMiddlewareBuilder{}
//	// 存储数据的，也就是你 userId 存哪里
//	// 直接存 cookie
//	store := cookie.NewStore([]byte("secret"))
//	// 基于内存的实现
//	//store := memstore.NewStore([]byte("k6CswdUm75WKcbM68UQUuxVsHSpTCwgK"),
//	//	[]byte("eF1`yQ9>yT1`tH1,sJ0.zD8;mZ9~nC6("))
//	//store, err := redis.NewStore(16, "tcp",
//	//	"localhost:6379", "",
//	//	[]byte("k6CswdUm75WKcbM68UQUuxVsHSpTCwgK"),
//	//	[]byte("k6CswdUm75WKcbM68UQUuxVsHSpTCwgA"))
//	//if err != nil {
//	//	panic(err)
//	//}
//	var jwt:=web.NewUserHandLer()
//	server.Use(sessions.Sessions("ssid", store), web.NewUserHandLer)
