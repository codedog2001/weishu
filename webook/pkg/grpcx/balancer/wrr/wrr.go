package wrr

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"sync"
)

// 该文件是负载均衡中，加权轮询的算法
const Name = "custom_weight_round_robin"

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, PickerBuilder{}, base.Config{})
}
func init() {
	balancer.Register(newBuilder())
}

type PickerBuilder struct {
}
type Picker struct {
	conns []*weightConn
	lock  sync.Mutex
}

// 实现了picker
// 这里写wrr的逻辑
func (p Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if len(p.conns) == 0 { //没有可用的节点，就不需要负载均衡
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	var total int
	for _, conn := range p.conns {
		total += conn.weight //计算总的初始
	}
	//注意这里是指针
	var maxCC *weightConn //最大权重的节点
	//计算当前的权重
	for _, conn := range p.conns {
		conn.currentWeight = conn.currentWeight + conn.weight
		//每一轮都要更新当前权重
		//当前权重=当前权重+初始权重
	}
	maxCC = p.conns[0] //默认第一个是最大权重
	for _, conn := range p.conns {
		//最大权重将会被选中，这里是> or >=都可以
		if maxCC == nil || conn.currentWeight > maxCC.currentWeight {
			maxCC = conn //因为maxcc是指针，maxcc和conn指向的是同一个地址
		}
	}
	//go语言可以自动解引用
	maxCC.currentWeight = maxCC.currentWeight - total
	return balancer.PickResult{
		SubConn: maxCC.SubConn,
	}, nil
}
func (p PickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	conns := make([]*weightConn, 0, len(info.ReadySCs))
	for sc, sci := range info.ReadySCs {
		md, _ := sci.Address.Metadata.(map[string]any)
		weight, _ := md["weight"].(int)
		conns = append(conns, &weightConn{
			SubConn:       sc,
			weight:        weight,
			currentWeight: weight,
		})
	}
	return &Picker{
		conns: conns,
	}
}

type weightConn struct {
	balancer.SubConn
	weight        int
	currentWeight int

	// 可以用来标记不可用
	available bool
}
