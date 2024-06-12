package grpcx

import (
	"context"
	etcdv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/netx"
)

// 这个文件用于包装GRPC服务端启动
type Server struct {
	*grpc.Server
	EtcdAddr    string //etcd地址
	Port        int    //grpc监听的端口
	Name        string //服务的名字
	etcdKey     string
	L           logger.LoggerV1
	EtcdClient  *etcdv3.Client
	etcdManager endpoints.Manager
	EtcdTTL     int64
	cancel      func()
}

// 装饰器模式
func (s *Server) Serve() error {
	//监听的自身的端口，所以不需要ip地址
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	port := strconv.Itoa(s.Port)
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	//完成注册
	err = s.register(ctx, port)
	if err != nil {
		panic(err)
	}
	//装饰器的体现，在原来的serve上加了以上的操作
	return s.Server.Serve(l) //启动grpc服务器
}

// 服务注册，   服务发现是在客户端进行
func (s *Server) register(ctx context.Context, port string) error {
	cli := s.EtcdClient
	serviceName := "service/" + s.Name
	em, err := endpoints.NewManager(cli,
		serviceName)
	if err != nil {
		return err
	}
	s.etcdManager = em
	ip := netx.GetOutboundIP()
	s.etcdKey = serviceName + "/" + ip
	addr := ip + ":" + port
	leaseResp, err := cli.Grant(ctx, s.EtcdTTL)
	// 开启续约
	ch, err := cli.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return err
	}
	go func() {
		// 可以预期，当我们的 cancel 被调用的时候，就会退出这个循环
		for chResp := range ch {
			s.L.Debug("续约：", logger.String("resp", chResp.String()))
		}
	}()
	// metadata 我们这里没啥要提供的
	return em.AddEndpoint(ctx, s.etcdKey,
		endpoints.Endpoint{Addr: addr}, etcdv3.WithLease(leaseResp.ID))
}

// 关闭服务
func (s *Server) Close() error {
	//一般这里都不会为空的，这里是为了防止手贱，乱调用
	if s.cancel != nil {
		s.cancel()
	}
	if s.EtcdClient != nil {
		return s.EtcdClient.Close()
	}
	//平滑关闭服务
	//首先跟注册中心说自己要退出了
	//然后不再接收新的请求且处理完已有的请求，然后退出
	s.GracefulStop()
	return nil
}
