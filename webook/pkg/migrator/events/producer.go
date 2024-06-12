package events

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
)

type Producer interface {
	ProduceInconsistentEvent(ctx context.Context, evt InconsistentEvent) error
}

// 实现了生产者接口
type SaramaProducer struct {
	p     sarama.SyncProducer
	topic string
}

func NewSaramaProducer(topic string, p sarama.SyncProducer) *SaramaProducer {
	return &SaramaProducer{
		p:     p,
		topic: topic,
	}
}
func (s *SaramaProducer) ProduceInconsistentEvent(ctx context.Context, evt InconsistentEvent) error {
	val, _ := json.Marshal(evt)
	_, _, err := s.p.SendMessage(&sarama.ProducerMessage{
		Topic: s.topic,
		Value: sarama.ByteEncoder(val),
	})
	//发送消息
	return err
}
