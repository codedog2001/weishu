package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
	"xiaoweishu/webook/internal/domain"
)

type ArticleCache interface {
	GetFirstPage(ctx context.Context, uid int64) ([]domain.Article, error)
	SetFirstPage(ctx context.Context, uid int64, res []domain.Article) error
	DelFirstPage(ctx context.Context, uid int64) error
	Get(ctx context.Context, id int64) (domain.Article, error)
	Set(ctx context.Context, art domain.Article) error
	GetPub(ctx context.Context, id int64) (domain.Article, error)
	SetPub(ctx context.Context, res domain.Article) error
	Like100(biz string) ([]domain.Like100, error)
	UpdateTopArticles(ctx context.Context, biz string, articles map[string]int64) error
}

type ArticleRedisCache struct {
	client redis.Cmdable
}

func (a ArticleRedisCache) UpdateTopArticles(ctx context.Context, biz string, articles map[string]int64) error {
	key := biz + "_likes"
	a.client.Del(ctx, key)

	for bizID, likes := range articles {
		a.client.ZAdd(ctx, key, redis.Z{Score: float64(likes), Member: bizID})
	}
	return nil
}

func (a ArticleRedisCache) Like100(biz string) ([]domain.Like100, error) {
	ctx := context.Background()
	key := biz + "_likes"
	likes, err := a.client.ZRevRangeWithScores(ctx, key, 0, 99).Result()
	if err != nil {
		return nil, err
	}
	var topLikes []domain.Like100
	for _, like := range likes {
		bizId, err := strconv.ParseInt(like.Member.(string), 10, 64)
		if err != nil {
			return nil, err
		}
		topLikes = append(topLikes, domain.Like100{
			BizId:   bizId,
			LikeCnt: int64(like.Score),
			Biz:     biz,
		})
	}
	return topLikes, nil
}

func (a ArticleRedisCache) GetFirstPage(ctx context.Context, uid int64) ([]domain.Article, error) {
	key := a.firstKey(uid)
	val, err := a.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var res []domain.Article
	err = json.Unmarshal(val, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a ArticleRedisCache) SetFirstPage(ctx context.Context, uid int64, arts []domain.Article) error {
	//当把第一页的文章都给存储到缓存中，内容是数据量很大，所以我们只需要存储摘要即可
	//前端也只展示摘要
	for i := 0; i < len(arts); i++ {
		arts[i].Content = arts[i].Abstract()
	}
	key := a.firstKey(uid)
	val, err := json.Marshal(arts)
	if err != nil {
		return err //将val序列成json格式失败
	}
	return a.client.Set(ctx, key, val, time.Minute*10).Err()
}

func (a ArticleRedisCache) DelFirstPage(ctx context.Context, uid int64) error {
	//TODO implement me
	panic("implement me")
}

//val := a.client.Get(ctx, a.key(id))
//那么val将不再是一个[]byte类型的值，而是a.client.Get方法返回的类型。
//假设a.client.Get返回的是一个实现了io.Reader接口的类型（这通常是缓存、数据库或HTTP客户端库中的常见做法），那么val将是一个io.Reader。
//
//这样的改动意味着你不能直接将val当作字节数组来处理，例如你不能直接使用json.Unmarshal(val, &res)，
//因为json.Unmarshal期望的第一个参数是一个字节切片[]byte，而不是io.Reader。
//
//为了使用val中的数据，你需要以流的方式读取它。如果你需要解析JSON，
//你需要使用json.NewDecoder(val).Decode(&res)来代替json.Unmarshal。json.NewDecoder接受一个io.Reader作为参数，
//并返回一个解码器，你可以使用这个解码器的Decode方法来从流中解析JSON。
//
//此外，如果a.client.Get返回的是一个io.ReadCloser，
//你还需要在读取完数据后调用Close()方法来释放相关资源。
//
//总结一下，改动后的代码将需要不同的处理方式来读取和使用val中的数据，特别是当涉及到解析JSON或其他格式的数据时。
//你需要使用流处理的方式，而不是直接操作字节数组。

func (a ArticleRedisCache) Get(ctx context.Context, id int64) (domain.Article, error) {
	val, err := a.client.Get(ctx, a.key(id)).Bytes()
	if err != nil {
		return domain.Article{}, err
	}
	var res domain.Article
	err = json.Unmarshal(val, &res)
	return res, nil
}

func (a ArticleRedisCache) Set(ctx context.Context, art domain.Article) error {
	//先把数据转成json格式在存储，
	//取数据的时候，先把json反序列化后再返回
	val, err := json.Marshal(art)
	if err != nil {
		return err
	}
	return a.client.Set(ctx, a.key(art.Id), val, time.Minute*10).Err()
}

func (a ArticleRedisCache) GetPub(ctx context.Context, id int64) (domain.Article, error) {
	val, err := a.client.Get(ctx, a.pubKey(id)).Bytes()
	if err != nil {
		return domain.Article{}, err
	}
	var res domain.Article
	err = json.Unmarshal(val, &res)
	if err != nil {
		return domain.Article{}, err
	}
	return res, err

}

func (a ArticleRedisCache) SetPub(ctx context.Context, res domain.Article) error {
	val, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return a.client.Set(ctx, a.pubKey(res.Id), val, time.Minute*10).Err()
}

func NewArticleRedisCache(client redis.Cmdable) ArticleCache {
	return &ArticleRedisCache{
		client: client,
	}
}
func (a *ArticleRedisCache) key(id int64) string {
	return fmt.Sprintf("article:detail:%d", id)
}
func (a *ArticleRedisCache) pubKey(id int64) string {
	return fmt.Sprintf("article:pub:detail:%d", id)
}
func (a *ArticleRedisCache) firstKey(uid int64) string {
	return fmt.Sprintf("article:first_page:%d", uid)
}
