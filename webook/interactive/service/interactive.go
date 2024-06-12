package service

import (
	"context"
	"golang.org/x/sync/errgroup"
	"xiaoweishu/webook/interactive/domain"
	"xiaoweishu/webook/interactive/repository"
	"xiaoweishu/webook/pkg/logger"
)

type InteractiveService interface {
	IncrReadCnt(ctx context.Context, biz string, bizId int64) error
	Like(c context.Context, biz string, id int64, uid int64) error
	CancelLike(c context.Context, biz string, id int64, uid int64) error
	Collect(ctx context.Context, biz string, bizId, cid, uid int64) error
	Get(ctx context.Context, biz string, id int64, uid int64) (domain.Interactive, error)
	GetByIds(ctx context.Context, biz string, ids []int64) (map[int64]domain.Interactive, error)
}

type interactiveService struct {
	repo repository.InteractiveRepository
	l    logger.LoggerV1
}

func (i interactiveService) GetByIds(ctx context.Context, biz string, ids []int64) (map[int64]domain.Interactive, error) {
	intrs, err := i.repo.GetByIds(ctx, biz, ids)
	if err != nil {
		return nil, err
	}
	res := make(map[int64]domain.Interactive, len(intrs))
	for _, intr := range intrs {
		res[intr.BizId] = intr
	}
	return res, nil

}

func (i interactiveService) IncrReadCnt(ctx context.Context, biz string, bizId int64) error {
	//通过不同的biz和不同的bizid就可以定位到一篇文章
	//比如有三个biz 视频图片文章，不同的biz领域的id是独立的，所以需要biz加bizid才能定位
	err := i.repo.IncrReadCnt(ctx, biz, bizId)
	if err != nil {
		return err
	}
	return nil
}

func (i interactiveService) Like(ctx context.Context, biz string, id int64, uid int64) error {
	err := i.repo.IncrLike(ctx, biz, id, uid)
	if err != nil {
		return err
	}
	return nil
}

func (i interactiveService) CancelLike(ctx context.Context, biz string, id int64, uid int64) error {
	err := i.repo.DecrLike(ctx, biz, id, uid)
	if err != nil {
		return err
	}
	return nil
}

func (i interactiveService) Collect(ctx context.Context, biz string, bizId, cid, uid int64) error {
	err := i.repo.AddCollectionItem(ctx, biz, bizId, cid, uid)
	if err != nil {
		return err
	}
	return nil
}

func (i interactiveService) Get(ctx context.Context, biz string, id int64, uid int64) (domain.Interactive, error) {
	intr, err := i.repo.Get(ctx, biz, id)
	if err != nil {
		return domain.Interactive{}, err
	}
	var eg errgroup.Group
	eg.Go(func() error {
		var er error
		intr.Liked, er = i.repo.Liked(ctx, biz, id, uid)
		return er
	})
	eg.Go(func() error {
		var er error
		intr.Collected, er = i.repo.Collected(ctx, biz, id, uid)
		return er
	})
	//若两个并发协程出错，其出错结果会放在eg.wait()中
	//谁先出错就放谁的
	err = eg.Wait()
	return intr, eg.Wait()

}

func NewInteractiveService(repo repository.InteractiveRepository) InteractiveService {
	return &interactiveService{
		repo: repo,
	}

}
