package ioc

import (
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"xiaoweishu/webook/internal/repository/dao"
	"xiaoweishu/webook/pkg/logger"
)

// 一般使用viper都会把配置解析到结构体中，而不是直接去使用
func InitDB(l logger.LoggerV1) *gorm.DB {
	type Config struct {
		DSN string `yaml:"dsn"`
	}
	var cfg Config = Config{
		DSN: "root:root@tcp(localhost:13316)/webook",
	} //先写一个默认值，如果没有配置文件，就使用这个默认值
	err := viper.UnmarshalKey("db", &cfg)
	if err != nil {
		panic(err)
	}
	//gorm使用自带的logger库，初始过程如下
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: glogger.New(goormLoggerFunc(l.Debug), glogger.Config{
			SlowThreshold: 0,
			LogLevel:      glogger.Info,
		}),
	})
	if err != nil {
		panic(err)
	}
	err = dao.InitTable(db)
	if err != nil {
		panic(err)
	}
	return db
}

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
