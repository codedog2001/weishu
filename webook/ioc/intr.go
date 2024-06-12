package ioc

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	etcdv3 "go.etcd.io/etcd/client/v3"
	resolver2 "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	intrv1 "xiaoweishu/webook/api/proto/gen/intr/v1"
	"xiaoweishu/webook/interactive/service"
	"xiaoweishu/webook/internal/client"
)

// 初始化用于interactive grpc客户端
func InitIntrClientV1(client *etcdv3.Client) intrv1.InteractiveServiceClient {
	type config struct {
		Addr   string `yaml:"addr"`
		Secure bool   `yaml:"secure"`
	}
	var cfg config
	err := viper.UnmarshalKey("grpc.client.intr", &cfg)
	if err != nil {
		panic(err)
	}
	resolver, err := resolver2.NewBuilder(client)
	if err != nil {
		panic(err)
	}
	opts := []grpc.DialOption{
		grpc.WithResolvers(resolver),
	}
	if !cfg.Secure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	cc, err := grpc.Dial(cfg.Addr, opts...)
	if err != nil {
		panic(err)
	}
	remote := intrv1.NewInteractiveServiceClient(cc)
	return remote //初始化远程客户端
	//这里已经用不上本地的客户端了，本地客户端只有在服务刚上线进行灰度发布的时候才会用到
}

// 初始化grpc客户端,用于刚拆分时的通信和灰度流量控制
func InitIntrClient(svc service.InteractiveService) intrv1.InteractiveServiceClient {
	type config struct {
		Addr      string `yaml:"addr"`
		Secure    bool
		Threshold int32
	}
	var cfg config
	err := viper.UnmarshalKey("grpc.client.intr", &cfg)
	if err != nil {
		panic(err)
	}
	var opts []grpc.DialOption
	if !cfg.Secure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	cc, err := grpc.Dial(cfg.Addr, opts...)
	if err != nil {
		panic(err)
	}
	remote := intrv1.NewInteractiveServiceClient(cc)       //初始化远程客户端
	local := client.NewLocalInteractiveServiceAdapter(svc) //初始化本地客户端
	res := client.NewInteractiveClient(remote, local)
	viper.OnConfigChange(func(in fsnotify.Event) {
		cfg = config{}
		err := viper.UnmarshalKey("grpc.client.intr", &cfg)
		if err != nil {
			panic(err)
		}
		res.UpdateThreshold(cfg.Threshold)
	})
	return res
}
