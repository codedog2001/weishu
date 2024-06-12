package events

// Consumer 消息队列消费者接口，理论上还需要实现stop方法
type Consumer interface {
	Start() error
}
