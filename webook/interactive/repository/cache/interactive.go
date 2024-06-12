package cache

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
	"xiaoweishu/webook/interactive/domain"
	"xiaoweishu/webook/interactive/repository/dao"
)

var (
	//go:embed lua/incr_cnt.lua
	luaIncrCnt string
)

const fieldReadCnt = "read_cnt"
const fieldLikeCnt = "like_cnt"
const fieldCollectCnt = "collect_cnt"

type InteractiveCache interface {
	IncrReadCntIfPresent(ctx context.Context, biz string, bizId int64) error
	IncrLikeCntIfPresent(ctx context.Context, biz string, id int64) error
	DecrLikeCntIfPresent(ctx context.Context, biz string, id int64) error
	IncrCollectCntIfPresent(ctx context.Context, biz string, id int64) error
	Get(ctx context.Context, biz string, id int64) (domain.Interactive, error)
	Set(ctx context.Context, biz string, bizId int64, res domain.Interactive) error
}
type InteractiveRedisCache struct {
	client redis.Cmdable
}

func NewInteractiveRedisCache(client redis.Cmdable) InteractiveCache {
	return &InteractiveRedisCache{
		client: client,
	}
}
func (i InteractiveRedisCache) IncrReadCntIfPresent(ctx context.Context, biz string, bizId int64) error {
	key := i.key(biz, bizId)
	//redis天然避免了并发，因为redis是单线程的
	return i.client.Eval(ctx, luaIncrCnt, []string{key}, fieldReadCnt, 1).Err()
}

func (i InteractiveRedisCache) IncrLikeCntIfPresent(ctx context.Context, biz string, id int64) error {
	key := i.key(biz, id)
	return i.client.Eval(ctx, luaIncrCnt, []string{key}, fieldLikeCnt, 1).Err()
}

func (i InteractiveRedisCache) DecrLikeCntIfPresent(ctx context.Context, biz string, biz_id int64) error {
	key := i.key(biz, biz_id)
	return i.client.Eval(ctx, luaIncrCnt, []string{key}, fieldLikeCnt, -1).Err()
}

func (i InteractiveRedisCache) IncrCollectCntIfPresent(ctx context.Context, biz string, id int64) error {
	key := i.key(biz, id)
	return i.client.Eval(ctx, luaIncrCnt, []string{key}, fieldCollectCnt, 1).Err()

}

func (i InteractiveRedisCache) Get(ctx context.Context, biz string, id int64) (domain.Interactive, error) {
	//当redis中某个键的值是一堆键值对时，那么存储的时候用哈希表进行存储更合适
	key := i.key(biz, id)
	res, err := i.client.HGetAll(ctx, key).Result()
	if err != nil {
		return domain.Interactive{}, err
	}
	if len(res) == 0 {
		return domain.Interactive{}, dao.ErrRecordNotFound
	}
	var intr domain.Interactive
	intr.Biz = biz
	intr.BizId = id
	intr.CollectCnt, _ = strconv.ParseInt(res[fieldCollectCnt], 10, 64)
	intr.LikeCnt, _ = strconv.ParseInt(res[fieldLikeCnt], 10, 64)
	intr.ReadCnt, _ = strconv.ParseInt(res[fieldReadCnt], 10, 64)
	return intr, nil
}

func (i InteractiveRedisCache) Set(ctx context.Context, biz string, bizId int64, res domain.Interactive) error {
	key := i.key(biz, bizId)
	//redis中若某个键的值是键值对的话，那么就用哈希表来存储，如果就是单独的一个值的话， 那么直接set /get就行了
	//否则就要用hset/hgetall
	err := i.client.HSet(ctx, key, fieldCollectCnt, res.CollectCnt,
		fieldLikeCnt, res.LikeCnt,
		fieldReadCnt, res.ReadCnt).Err()
	//这里也要写成哈希表的样子
	if err != nil {
		return err //回写缓存失败
	}
	return i.client.Expire(ctx, key, time.Minute*15).Err()
}

func (i *InteractiveRedisCache) key(biz string, bizId int64) string {
	return fmt.Sprintf("interactive:%s:%d", biz, bizId)
}
