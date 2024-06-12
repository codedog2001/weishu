package repository

import (
	"context"
	"errors"
	"github.com/ecodeclub/ekit/slice"
	"xiaoweishu/webook/interactive/domain"
	"xiaoweishu/webook/interactive/repository/cache"
	dao "xiaoweishu/webook/interactive/repository/dao"
	logger2 "xiaoweishu/webook/pkg/logger"
)

type InteractiveRepository interface {
	IncrReadCnt(ctx context.Context, biz string, bizId int64) error
	IncrLike(ctx context.Context, biz string, id int64, uid int64) error
	DecrLike(ctx context.Context, biz string, id int64, uid int64) error
	AddCollectionItem(ctx context.Context, biz string, id int64, cid int64, uid int64) error
	Get(ctx context.Context, biz string, id int64) (domain.Interactive, error)
	Liked(ctx context.Context, biz string, id int64, uid int64) (bool, error)
	Collected(ctx context.Context, biz string, id int64, uid int64) (bool, error)
	BatchIncrReadCnt(ctx context.Context, bizs []string, ids []int64) error
	GetByIds(ctx context.Context, biz string, ids []int64) ([]domain.Interactive, error)
}
type CachedInteractiveRepository struct {
	dao   dao.InteractiveDAO
	cache cache.InteractiveCache
	l     logger2.LoggerV1
}

func (c *CachedInteractiveRepository) GetByIds(ctx context.Context, biz string, ids []int64) ([]domain.Interactive, error) {
	intrs, err := c.dao.GetByIds(ctx, biz, ids)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.Interactive, domain.Interactive](intrs, func(idx int, src dao.Interactive) domain.Interactive {
		return c.ToDomain(src)
	}), nil

}

func (c *CachedInteractiveRepository) BatchIncrReadCnt(ctx context.Context, bizs []string, bizIds []int64) error {
	err := c.dao.BatchIncrReadCnt(ctx, bizs, bizIds)
	if err != nil {
		return err
	}
	//到这说明数据库更新成功，可以开启异步更新redis缓存
	go func() {
		for i := 0; i < len(bizs); i++ {
			er := c.cache.IncrReadCntIfPresent(ctx, bizs[i], bizIds[i])
			if err != nil {
				c.l.Debug("批量缓存阅读数失败", logger2.Error(er))
			}
		}
	}()
	//这里不需要等待redis缓存完，因为就算是缓存失败也不是什么大事
	return nil
}

func (c *CachedInteractiveRepository) IncrReadCnt(ctx context.Context, biz string, bizId int64) error {
	err := c.dao.IncrReadCnt(ctx, biz, bizId)
	if err != nil {
		return err
	}
	//这时候需要设置缓存，否则会导致数据不一致问题
	err = c.cache.IncrReadCntIfPresent(ctx, biz, bizId)
	if err != nil {
		return err
	}
	return nil
}

func (c *CachedInteractiveRepository) IncrLike(ctx context.Context, biz string, id int64, uid int64) error {
	//一个人只能点赞一次，所以点赞的时候直接插入点赞的表格即可
	//再次点赞就更新UTime即可
	err := c.dao.InsertLikeInfo(ctx, biz, id, uid)
	if err != nil {
		return err
	}
	return c.cache.IncrLikeCntIfPresent(ctx, biz, id)
}

func (c *CachedInteractiveRepository) DecrLike(ctx context.Context, biz string, id int64, uid int64) error {
	err := c.dao.DeleteLikeInfo(ctx, biz, id, uid)
	if err != nil {
		return err
	}
	return c.cache.DecrLikeCntIfPresent(ctx, biz, id)
}

func (c *CachedInteractiveRepository) AddCollectionItem(ctx context.Context, biz string, id int64, cid int64, uid int64) error {
	var res = dao.UserCollectionBiz{
		Uid:   uid,
		BizId: id,
		Biz:   biz,
		Cid:   cid,
	}
	err := c.dao.InsertCollectionBiz(ctx, res)
	if err != nil {
		return err
	}
	return c.cache.IncrCollectCntIfPresent(ctx, biz, id)
}

func (c *CachedInteractiveRepository) Get(ctx context.Context, biz string, id int64) (domain.Interactive, error) {
	intr, err := c.cache.Get(ctx, biz, id)
	if err == nil {
		return intr, err
	}
	ie, err := c.dao.Get(ctx, biz, id)
	if err != nil {
		return domain.Interactive{}, err
	}
	res := c.ToDomain(ie)
	//回写缓存
	err = c.cache.Set(ctx, biz, id, res)
	if err != nil {
		c.l.Error("回写缓存失败", logger2.String("biz", biz),
			logger2.Int64("bizId", id),
			logger2.Error(err))
	}
	return res, nil

}

// 关于用户喜欢的逻辑，这里定义成，若用户喜欢，那么就会生成喜欢的表，取消喜欢赞时，就会把对应的表格删除
// 所以只要dao层能找到该表，那就表明了用户点赞，否则就是没有点赞
func (c *CachedInteractiveRepository) Liked(ctx context.Context, biz string, id int64, uid int64) (bool, error) {
	_, err := c.dao.GetLikeInfo(ctx, biz, id, uid)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, dao.ErrRecordNotFound):
		return false, nil //没找到说明，没有点赞，不需要返回错误
	default:
		return false, err
	}
}

func (c *CachedInteractiveRepository) Collected(ctx context.Context, biz string, id int64, uid int64) (bool, error) {
	_, err := c.dao.GetCollectInfo(ctx, biz, id, uid)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, dao.ErrRecordNotFound):
		return false, nil //没有找到，说明没有进行收藏，不需要进行返回错误
	default:
		return false, err
	}
}

func NewCachedInteractiveRepository(dao dao.InteractiveDAO,
	cache cache.InteractiveCache,
	l logger2.LoggerV1) InteractiveRepository {
	return &CachedInteractiveRepository{
		dao:   dao,
		cache: cache,
		l:     l,
	}

}
func (c *CachedInteractiveRepository) ToDomain(ie dao.Interactive) domain.Interactive {
	return domain.Interactive{
		Biz:        ie.Biz,
		BizId:      ie.BizId,
		ReadCnt:    ie.ReadCnt,
		CollectCnt: ie.CollectCnt,
		LikeCnt:    ie.LikeCnt,
	}
}
