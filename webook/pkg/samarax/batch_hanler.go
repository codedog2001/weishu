package samarax

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"time"
	logger2 "xiaoweishu/webook/pkg/logger"
)

type BatchHandler[T any] struct {
	fn func(msgs []*sarama.ConsumerMessage, ts []T) error
	l  logger2.LoggerV1
}

func NewBatchHandler[T any](l logger2.LoggerV1, fn func(msgs []*sarama.ConsumerMessage, ts []T) error) *BatchHandler[T] {
	return &BatchHandler[T]{
		fn: fn,
		l:  l,
	}
}

func (b *BatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b *BatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim session 是 Kafka 消费者组的会话，而 claim 是消费者组中的一个分区（或称为“claim”）。
func (b *BatchHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgs := claim.Messages()
	const batchSize = 10
	for {
		batch := make([]*sarama.ConsumerMessage, 0, batchSize)
		ts := make([]T, 0, batchSize)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var done = false
		for i := 0; i < batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				//超时了
				done = true
			//这时就不会继续循环，跳出循环
			case msg, ok := <-msgs:
				if !ok {
					cancel()
					return nil
				}
				batch = append(batch, msg)
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					b.l.Error("反序列化失败",
						logger2.String("topic", msg.Topic),
						logger2.Int32("partition", msg.Partition),
						logger2.Int64("offset", msg.Offset),
						logger2.Error(err))
					continue //继续下一条消息
				}
				batch = append(batch, msg)
				ts = append(ts, t)
			}
		}
		//每次循环都会创建新的带超时控制的上下文
		//不论是到批次了还是超时了，都会取消这个context
		//超时控制：设置超时是为了防止在 Kafka 没有新消息时，消费者线程无限期地等待。
		//超时后，即使批次中的消息数量未达到 batchSize，也会结束当前批次的处理，并开始下一个批次。
		//这有助于保持消费者的响应性，避免资源（如线程）的长时间占用
		cancel()
		//凑到一批开始处理消息
		err := b.fn(batch, ts)
		if err != nil {
			b.l.Error("处理消息失败", logger2.Error(err))
		}
		for _, msg := range batch {
			session.MarkMessage(msg, "")
			//对已经处理过的消息进行标记，kafka将不再对这些消息进行通知
		}
	}
}
