package ioc

import (
	"github.com/spf13/viper"
	etcdv3 "go.etcd.io/etcd/client/v3"
)

// InitEtcd 客户端和服务端与etcd进行通信
func InitEtcd() *etcdv3.Client {
	//通常会部署一个 etcd 集群，而不是单个 etcd 实例。etcd 集群提供了更高的可用性和容错能力。
	//因此，客户端初始化 etcd 时，使用的是一个地址切片 (Addrs)，其中包含多个 etcd 节点的地址。
	var cfg etcdv3.Config
	err := viper.UnmarshalKey("etcd", &cfg)
	if err != nil {
		panic(err)
	}
	client, err := etcdv3.New(cfg)
	if err != nil {
		panic(err)
	}
	return client
}
