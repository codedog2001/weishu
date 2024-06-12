package grpc

import (
	"context"
	"google.golang.org/grpc"
	intrv1 "xiaoweishu/webook/api/proto/gen/intr/v1"
	"xiaoweishu/webook/interactive/domain"
	"xiaoweishu/webook/interactive/service"
)

//通过buf编译已经生成了grpc接口定义，这里进行实现
//实现grpc接口

type InteractiveServiceServer struct {
	intrv1.UnimplementedInteractiveServiceServer
	svc service.InteractiveService
}

func (i *InteractiveServiceServer) Register(s *grpc.Server) {
	intrv1.RegisterInteractiveServiceServer(s, i) //注册grpc服务端
}

func NewInteractiveServiceServer(svc service.InteractiveService) *InteractiveServiceServer {
	return &InteractiveServiceServer{svc: svc}
}

func (i *InteractiveServiceServer) IncrReadCnt(ctx context.Context, request *intrv1.IncrReadCntRequest) (*intrv1.IncrReadCntResponse, error) {
	//相当于把grpc通信得到的任务，转为微服务模块中的进行执行
	err := i.svc.IncrReadCnt(ctx, request.GetBiz(), request.GetBizId())
	return &intrv1.IncrReadCntResponse{}, err
}

func (i *InteractiveServiceServer) Like(ctx context.Context, request *intrv1.LikeRequest) (*intrv1.LikeResponse, error) {
	err := i.svc.Like(ctx, request.GetBiz(), request.GetBizId(), request.GetUid())
	return &intrv1.LikeResponse{}, err
}

func (i *InteractiveServiceServer) CancelLike(ctx context.Context, request *intrv1.CancelLikeRequest) (*intrv1.CancelLikeResponse, error) {
	err := i.svc.CancelLike(ctx, request.GetBiz(), request.GetBizId(), request.GetUid())
	return &intrv1.CancelLikeResponse{}, err
}

func (i *InteractiveServiceServer) Collect(ctx context.Context, request *intrv1.CollectRequest) (*intrv1.CollectResponse, error) {
	err := i.svc.Collect(ctx, request.GetBiz(), request.GetBizId(), request.GetCid(), request.GetUid())
	return &intrv1.CollectResponse{}, err
}

func (i *InteractiveServiceServer) Get(ctx context.Context, request *intrv1.GetRequest) (*intrv1.GetResponse, error) {
	intr, err := i.svc.Get(ctx, request.GetBiz(), request.GetBizId(), request.GetUid())
	if err != nil {
		return nil, err
	}
	return &intrv1.GetResponse{
		Intr: i.toDTO(intr),
	}, nil
}

func (i *InteractiveServiceServer) GetByIds(ctx context.Context, request *intrv1.GetByIdsRequest) (*intrv1.GetByIdsResponse, error) {
	res, err := i.svc.GetByIds(ctx, request.GetBiz(), request.GetIds())
	if err != nil {
		return nil, err
	}
	intrs := make(map[int64]*intrv1.Interactive, len(res))
	for k, v := range res {
		intrs[k] = i.toDTO(v)
	}
	return &intrv1.GetByIdsResponse{
		Intrs: intrs,
	}, nil
}

func (i *InteractiveServiceServer) mustEmbedUnimplementedInteractiveServiceServer() {
	//TODO implement me
	panic("implement me")
}

// 把领域对象转换成grpc中的定义，必选要转，哪怕类型一样，通信的语言不同
func (i *InteractiveServiceServer) toDTO(intr domain.Interactive) *intrv1.Interactive {
	return &intrv1.Interactive{
		Biz:        intr.Biz,
		BizId:      intr.BizId,
		ReadCnt:    intr.ReadCnt,
		CollectCnt: intr.CollectCnt,
		Collected:  intr.Collected,
		Liked:      intr.Liked,
		LikeCnt:    intr.LikeCnt,
	}
}
