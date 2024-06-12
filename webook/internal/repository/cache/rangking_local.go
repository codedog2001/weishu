package cache

import (
	"context"
	"errors"
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"time"
	"xiaoweishu/webook/internal/domain"
)

type RankingLocalCache struct {
	topN       *atomicx.Value[[]domain.Article]
	ddl        *atomicx.Value[time.Time] //过期时间 now +expiration
	expiration time.Duration
}

func (r *RankingLocalCache) Set(ctx context.Context, arts []domain.Article) error {
	r.topN.Store(arts)
	r.ddl.Store(time.Now().Add(r.expiration))
	return nil

}

func (r *RankingLocalCache) Get(ctx context.Context) ([]domain.Article, error) {
	ddl := r.ddl.Load()
	arts := r.topN.Load()
	if len(arts) == 0 || ddl.Before(time.Now()) {
		return nil, errors.New("本地缓存失效了")
	}
	return arts, nil
}

func (r *RankingLocalCache) ForceGet(ctx context.Context) ([]domain.Article, error) {

	arts := r.topN.Load()
	//这种情况一般出现本地缓存过期，redis崩了的情况下
	if len(arts) == 0 {
		return nil, errors.New("本地换吃失效，连过期的数据也没有")
	}
	return arts, nil
}

//不能redis和local同时new，这样会造成接口重复，只能和V1一样，。使用combined interfaced来解决

func NewRankingLocalCache(topN *atomicx.Value[[]domain.Article],
	ddl *atomicx.Value[time.Time], expiration time.Duration) RankingCache {
	return &RankingLocalCache{
		topN:       topN,
		ddl:        ddl,
		expiration: expiration,
	}
}
