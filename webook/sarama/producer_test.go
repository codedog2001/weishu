package sarama

import (
	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"testing"
)

var addr = []string{"localhost:9094"}

func TestSyncProducer(t *testing.T) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(addr, cfg)
	cfg.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	//cfg.Producer.Partitioner = sarama.NewRandomPartitioner
	//cfg.Producer.Partitioner = sarama.NewHashPartitioner
	//cfg.Producer.Partitioner = sarama.NewManualPartitioner
	//cfg.Producer.Partitioner = sarama.NewConsistentCRCHashPartitioner
	//cfg.Producer.Partitioner = sarama.NewCustomPartitioner()
	assert.NoError(t, err)
	for i := 0; i < 1000; i++ {
		_, _, err = producer.SendMessage(&sarama.ProducerMessage{
			//这个topic很重要，生产者和消费者的默契就是这个topic名字
			Topic: "article_read",
			Value: sarama.StringEncoder(`{"aid":1,"uid":123}`),
			// 会在生产者和消费者之间传递的
		})
	}
}

func TestAsyncProducer(t *testing.T) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	producer, err := sarama.NewAsyncProducer(addr, cfg)
	assert.NoError(t, err)
	msgs := producer.Input()
	msgs <- &sarama.ProducerMessage{
		Topic: "test_topic",
		Value: sarama.StringEncoder("这是一条消息"),
		// 会在生产者和消费者之间传递的
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("key1"),
				Value: []byte("value1"),
			},
		},
		Metadata: "这是 metadata",
	}

	select {
	case msg := <-producer.Successes():
		t.Log("发送成功", string(msg.Value.(sarama.StringEncoder)))
	case err := <-producer.Errors():
		t.Log("发送失败", err.Err, err.Msg)
	}
}
