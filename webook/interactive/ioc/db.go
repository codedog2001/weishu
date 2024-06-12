package ioc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
	prometheus2 "gorm.io/plugin/prometheus"
	"xiaoweishu/webook/interactive/repository/dao"
	"xiaoweishu/webook/pkg/gormx"
	"xiaoweishu/webook/pkg/gormx/connpool"
	"xiaoweishu/webook/pkg/logger"
)

type SrcDB *gorm.DB
type DstDB *gorm.DB

func InitSrcDB() SrcDB {
	return InitDB("src")
}

func InitDstDB() DstDB {
	return InitDB("dst")
}
func InitDoubleWritePool(src SrcDB, dst DstDB, l logger.LoggerV1) *connpool.DoubleWritePool {
	return connpool.NewDoubleWritePool(src, dst, l)
}

//业务数据库 (bizDB)：
//不是一个独立的实体数据库，而是一个 GORM 的数据库连接对象。
//它使用双写池 (DoubleWritePool) 来管理和执行对源数据库和目标数据库的操作
//用双写池管理和协调对两个底层实体数据库（源数据库和目标数据库）的操作。
//这样设计的目的是提供数据冗余和一致性，提高系统的可靠性和容错能力。
// 一般使用viper都会把配置解析到结构体中，而不是直接去使用

func InitBizDB(p *connpool.DoubleWritePool) *gorm.DB {
	doubleWrite, err := gorm.Open(mysql.New(mysql.Config{
		Conn: p,
	}))
	if err != nil {
		panic(err)
	}
	return doubleWrite
}
func InitDB(key string) *gorm.DB {
	type Config struct {
		DSN string `yaml:"dsn"`
	}
	var cfg Config = Config{
		DSN: "",
	} //先写一个默认值，如果没有配置文件，就使用这个默认值
	err := viper.UnmarshalKey("db."+key, &cfg)
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	err = db.Use(prometheus2.New(prometheus2.Config{
		DBName:          "webook" + key,
		RefreshInterval: 15,
		MetricsCollector: []prometheus2.MetricsCollector{
			&prometheus2.MySQL{
				VariableNames: []string{"thread_running"},
			},
		},
	}))
	if err != nil {
		panic(err)
	}

	cb := gormx.NewCallbacks(prometheus.SummaryOpts{
		Namespace: "ZHAOXIAO",
		Name:      "gorm_db",
		Subsystem: "webook",
		Help:      "统计GORM的数据库查询",
		ConstLabels: map[string]string{
			"instance_id": "my_instance",
		},
		Objectives: map[float64]float64{
			0.5:   0.01,
			0.75:  0.01,
			0.9:   0.01,
			0.99:  0.001,
			0.999: 0.0001,
		},
	})

	err = cb.Initialize(db)
	if err != nil {
		panic(err)
	}

	err = db.Use(tracing.NewPlugin(tracing.WithoutMetrics(), tracing.WithDBName("webook"))) // 跟踪的数据库名字
	if err != nil {
		panic(err)
	}

	err = dao.InitTables(db)
	if err != nil {
		panic(err)
	}

	return db
}

//func InitDB(key string) *gorm.DB {
//	type Config struct {
//		DSN string `yaml:"dsn"`
//	}
//	var cfg Config = Config{
//		DSN: "root:root@tcp(localhost:13316)/webook",
//	} //先写一个默认值，如果没有配置文件，就使用这个默认值
//	err := viper.UnmarshalKey("db"+key, &cfg)
//	if err != nil {
//		panic(err)
//	}
//	//gorm使用自带的logger库，初始过程如下
//	//db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
//	//	Logger: glogger.New(goormLoggerFunc(l.Debug), glogger.Config{
//	//		SlowThreshold: 0,
//	//		LogLevel:      glogger.Info,
//	//	}),
//	//})
//	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
//	if err != nil {
//		panic(err)
//	}
//	err = db.Use(prometheus2.New(prometheus2.Config{
//		DBName:          "webook" + key,
//		RefreshInterval: 15,
//		MetricsCollector: []prometheus2.MetricsCollector{
//			&prometheus2.MySQL{
//				VariableNames: []string{"thread_running"},
//			},
//		},
//	}))
//	if err != nil {
//		panic(err)
//	}
//	cb := gormx.NewCallbacks(prometheus.SummaryOpts{
//		Namespace: "ZHAOXIAO",
//		Name:      "gorm_db",
//		Subsystem: "webook",
//		Help:      "统计GORM的数据库查询",
//		ConstLabels: map[string]string{
//			"instance_id": "my_instance",
//		},
//		Objectives: map[float64]float64{
//			0.5:   0.01,
//			0.75:  0.01,
//			0.9:   0.01,
//			0.99:  0.001,
//			0.999: 0.0001,
//		},
//	})
//	//注册了所有的钩子函数，否则要自己一个个写
//	//类似于db.Callback().Create().Before("*"). Register("prometheus_create_before",func())
//	//gormx也就是把这类函数做了封装，核心仍是这些函数
//	err = cb.Initialize(db)
//	if err != nil {
//		return nil
//	}
//	//不需要记录metrics，metrics由prometheus负责
//	err = db.Use(tracing.NewPlugin(tracing.WithoutMetrics(),
//		tracing.WithDBName("webook"))) //跟踪的数据库名字
//	if err != nil {
//		panic(err)
//	}
//	err = db.Use(cb)
//	if err != nil {
//		panic(err)
//	}
//	//注册所用的钩子函数
//	err = dao.InitTables(db)
//	if err != nil {
//		panic(err)
//	}
//	return db
//}

type goormLoggerFunc func(msg string, fields ...logger.Field)

func (g goormLoggerFunc) Printf(s string, i ...interface{}) {
	g(s, logger.Field{Key: "args", Val: i})
}

//外部logger (l logger.LoggerV1):
//这个logger是作为函数参数传入的，类型为logger.LoggerV1。它代表的是项目或应用程序级别的日志系统，
//通常用于记录整个程序运行过程中的各种事件、调试信息、错误信息等。这种logger通常是全局的、
//统一的，具有标准化的日志格式和输出目的地（如控制台、文件、远程日志服务等），便于开发者集中管理和监控应用的整体状态。
//GORM内部logger (glogger.New(goormLoggerFunc(l.Debug), glogger.Config...)):
//这部分是在初始化gorm.DB实例时，通过&gorm.Config{Logger: ...}设置的。它特指GORM框架自身的日志系统，
//用于记录与数据库交互相关的详细信息，如SQL查询语句、执行时间、错误信息等。这些日志有助于调试ORM操作、
//优化查询性能，以及在出现数据库问题时快速定位原因。
//在这里，创建了一个glogger.New实例，其内部使用了自定义的goormLoggerFunc类型作为日志处理器。
//这个处理器实际上是对传入的外部logger(l)的Debug方法的一个适配，使得GORM产生的数据库日志能够通过l.Debug方法输出。
//goormLoggerFunc实现了Printf方法，将GORM的格式化日志消息转换为logger.Field形式，传递给外部logger。
//总结来说，两个logger分工明确：
//外部logger：负责整个应用程序的通用日志记录，涵盖所有模块和组件的行为。
//GORM内部logger：专注于记录与数据库交互相关的详细信息，通过自定义适配器桥接至外部logger
//，确保这些特定领域的日志能够整合到项目的统一日志体系中，同时保持GORM日志的特有格式和内容。
