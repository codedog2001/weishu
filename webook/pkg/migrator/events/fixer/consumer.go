package fixer

import (
	"context"
	"errors"
	"github.com/IBM/sarama"
	"gorm.io/gorm"
	"time"
	"xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/migrator"
	"xiaoweishu/webook/pkg/migrator/events"
	"xiaoweishu/webook/pkg/migrator/fixer"
	"xiaoweishu/webook/pkg/samarax"
)

// 修复的时候要考虑以谁为准的问题
// 特别是双写阶段
// 一开始全量校验肯定是以源库为准，而不是迁移的那个数据库
type Consumer[T migrator.Entity] struct {
	client   sarama.Client
	l        logger.LoggerV1
	srcFirst *fixer.OverrideFixer[T]
	dstFirst *fixer.OverrideFixer[T]
	topic    string
}

func NewConsumer[T migrator.Entity](
	client sarama.Client,
	l logger.LoggerV1,
	src *gorm.DB,
	dst *gorm.DB,
	topic string) (*Consumer[T], error) {
	//以源库为准，进行fixer的初始化
	srcFirst, err := fixer.NewOverrideFixer[T](src, dst)
	if err != nil {
		return nil, err
	}
	//以目标库为准，进行fixer的初始化
	dstFirst, err := fixer.NewOverrideFixer[T](dst, src)
	if err != nil {
		return nil, err
	}
	return &Consumer[T]{
		client:   client,
		l:        l,
		srcFirst: srcFirst,
		dstFirst: dstFirst,
		topic:    topic,
	}, nil
}

// 在main函数中挂着start函数，用来接收消息
func (r *Consumer[T]) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("migrator-fix", r.client)
	if err != nil {
		return err
	}
	go func() {
		err := cg.Consume(context.Background(), []string{r.topic}, //从指定分区中消费消息
			samarax.NewHandler[events.InconsistentEvent](r.l, r.Consume))
		if err != nil {
			r.l.Error("消费消息失败", logger.Error(err))
		}
	}()
	return err
}

// 消费信息，用于数据不一致的时候进行修复
// 消息中会带着方向，根据方向来判断fix方向和base
func (r *Consumer[T]) Consume(msg *sarama.ConsumerMessage, t events.InconsistentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	switch t.Direction { //direction是指在以为哪个表为base的时候出现不一致的情况，
	// 然后发的kafka，所以这时候校验仍然是要以哪个表为base为准
	case "SRC":
		return r.srcFirst.Fix(ctx, t.ID)
	case "DST":
		return r.dstFirst.Fix(ctx, t.ID)
	}
	return errors.New("未知的校验方向")
}
