package cache

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"time"
	"xiaoweishu/webook/internal/domain"
)

type RankingCache interface {
	Set(ctx context.Context, arts []domain.Article) error
	Get(ctx context.Context) ([]domain.Article, error)
}
type RankingRedisCache struct {
	client     redis.Cmdable
	key        string
	expiration time.Duration
}

func (r RankingRedisCache) Set(ctx context.Context, arts []domain.Article) error {
	for _, art := range arts {
		art.Content = art.Abstract()
	}
	//存的时候记得先序列化，取出来之后记得先反序列化
	val, err := json.Marshal(arts)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, r.key, val, r.expiration).Err()
}

func (r RankingRedisCache) Get(ctx context.Context) ([]domain.Article, error) {
	val, err := r.client.Get(ctx, r.key).Bytes()
	if err != nil {
		return nil, err
	}
	var res []domain.Article
	err = json.Unmarshal(val, &res)
	return res, nil
}

func NewRankingRedisCache(client redis.Cmdable) RankingCache {
	return &RankingRedisCache{
		client:     client,
		key:        "ranking:top_n",
		expiration: time.Minute * 30,
		//为了保险起见，这个过期时间可以设置的很长，也可以设置成永不过期，这样只要是redis没蹦的情况下就会访问到数据
		//如果新的热榜没有出来，而且旧的数据已经过期，那么就不再关心时间是否过期
		//只要有数据就先返回回去，热榜的短时间内不会有很大变化
	}
}
