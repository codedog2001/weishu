package events

import (
	"context"
	"github.com/IBM/sarama"
	"time"
	"xiaoweishu/webook/interactive/repository"
	"xiaoweishu/webook/internal/events/article"
	logger2 "xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/samarax"
)

// 读事件分区名字，interactive读写消息都在这个分区里
const TopicReadEvent = "article_read"

// 装饰器模式
type InteractiveReadEventConsumer struct {
	repo   repository.InteractiveRepository
	client sarama.Client
	l      logger2.LoggerV1
}

func (i *InteractiveReadEventConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("interactive", i.client)
	if err != nil {
		return err
	}
	go func() {
		er := cg.Consume(context.Background(), []string{TopicReadEvent},
			samarax.NewBatchHandler[article.ReadEvent](i.l, i.BatchConsume))
		if er != nil {
			i.l.Error("退出消费", logger2.Error(er))
		}
	}()
	return err
}

func NewInteractiveReadEventConsumer(repo repository.InteractiveRepository,
	client sarama.Client, l logger2.LoggerV1) *InteractiveReadEventConsumer {
	return &InteractiveReadEventConsumer{
		repo:   repo,
		client: client,
		l:      l,
	}
}
func (i *InteractiveReadEventConsumer) BatchConsume(msgs []*sarama.ConsumerMessage,
	events []article.ReadEvent) error {
	bizs := make([]string, 0, len(events))
	bizIds := make([]int64, 0, len(events))
	for _, evt := range events {
		//通过biz和bizid定位到具体的文章
		bizs = append(bizs, "article")
		bizIds = append(bizIds, evt.Aid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return i.repo.BatchIncrReadCnt(ctx, bizs, bizIds)
}
