package ioc

import (
	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"xiaoweishu/webook/interactive/repository/dao"
	"xiaoweishu/webook/pkg/ginx"
	"xiaoweishu/webook/pkg/gormx/connpool"
	"xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/migrator/events"
	"xiaoweishu/webook/pkg/migrator/events/fixer"
	"xiaoweishu/webook/pkg/migrator/scheduler"
)

func InitGinxSever(l logger.LoggerV1,
	src SrcDB,
	dst DstDB,
	pool *connpool.DoubleWritePool,
	producer events.Producer) *ginx.Server {
	engine := gin.Default()
	group := engine.Group("/migrator")
	//初始化计数器
	ginx.InitCounter(prometheus.CounterOpts{
		Namespace: "geektime_daming",
		Subsystem: "webook_intr_admin",
		Name:      "biz_code",
		Help:      "统计业务错误码",
	})
	sch := scheduler.NewScheduler[dao.Interactive](l, src, dst, producer, pool)
	sch.RegisterRoutes(group)
	return &ginx.Server{
		Engine: engine,
		Addr:   viper.GetString("migrator.http.addr"), //不停机服务的的专用接口
	}
}
func InitInteractiveProducer(p sarama.SyncProducer) events.Producer {
	return events.NewSaramaProducer("inconsistent_interactive", p)
}
func InitFixerConsumer(client sarama.Client,
	l logger.LoggerV1,
	src SrcDB,
	dst DstDB) *fixer.Consumer[dao.Interactive] {
	res, err := fixer.NewConsumer[dao.Interactive](client, l, src, dst, "inconsistent_interactive")
	if err != nil {
		panic(err)
	}
	return res
}
