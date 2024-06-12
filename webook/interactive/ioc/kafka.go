package ioc

import (
	"github.com/IBM/sarama"
	"github.com/spf13/viper"
	events2 "xiaoweishu/webook/interactive/events"
	"xiaoweishu/webook/interactive/repository/dao"
	"xiaoweishu/webook/internal/events"
	"xiaoweishu/webook/pkg/migrator/events/fixer"
)

func InitSaramaClient() sarama.Client {
	type Config struct {
		Addr []string `yaml:"addr"`
	}
	var cfg Config
	err := viper.UnmarshalKey("kafka", &cfg)
	if err != nil {
		panic(err)
	}
	scfg := sarama.NewConfig()
	scfg.Producer.Return.Successes = true
	client, err := sarama.NewClient(cfg.Addr, scfg)
	if err != nil {
		panic(err)
	}
	return client
}

// 初始化同步生产者，同步生产者更看重顺序，会导致消息到达的一致性，异步则不能保证消息到达的一致性
func InitSaramaSyncProducer(client sarama.Client) sarama.SyncProducer {
	p, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic(err)
	}
	return p
}

// 每种事件都需要初始化一个消费者
func InitConsumers(c1 *events2.InteractiveReadEventConsumer, fixConsumer *fixer.Consumer[dao.Interactive]) []events.Consumer {
	return []events.Consumer{c1, fixConsumer}
}
