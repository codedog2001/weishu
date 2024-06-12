package repository

import (
	"context"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository/cache"
	"xiaoweishu/webook/internal/repository/dao"
	"xiaoweishu/webook/pkg/logger"
)

type ArticleRepository interface {
	Create(ctx context.Context, art domain.Article) (int64, error)
	Update(ctx context.Context, art domain.Article) error
	Sync(ctx context.Context, art domain.Article) (int64, error)
	SyncStatus(ctx context.Context, uid int64, id int64, status domain.ArticleStatus) error
	GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error)
	GetById(ctx context.Context, id int64) (domain.Article, error)
	GetPubById(ctx context.Context, id int64) (domain.Article, error)
	ListPub(ctx context.Context, start time.Time, offset int, limit int) ([]domain.Article, error)
	Like100(ctx *gin.Context, biz string) ([]domain.Like100, error)
	GetTopArticles(ctx context.Context, biz string, number int) error
}

type CachedArticleRepository struct {
	dao      dao.ArticleDAO
	db       *gorm.DB
	cache    cache.ArticleCache
	userRepo UserRepository
}

func (c *CachedArticleRepository) GetTopArticles(ctx context.Context, biz string, number int) error {
	articles, err := c.dao.GetTopArticles(ctx, biz, number)
	if err != nil {
		return err
	}
	return c.cache.UpdateTopArticles(ctx, biz, articles)
}

func (c *CachedArticleRepository) Like100(ctx *gin.Context, biz string) ([]domain.Like100, error) {
	return c.cache.Like100(biz)
}

func NewCachedArticleRepository(dao dao.ArticleDAO,
	userRepo UserRepository,
	cache cache.ArticleCache) ArticleRepository {
	return &CachedArticleRepository{
		dao:      dao,
		cache:    cache,
		userRepo: userRepo,
	}
}
func (c *CachedArticleRepository) ListPub(ctx context.Context, start time.Time, offset int, limit int) ([]domain.Article, error) {
	arts, err := c.dao.ListPub(ctx, start, offset, limit)
	if err != nil {
		return nil, err
	}
	return slice.Map[dao.PublishedArticle, domain.Article](arts,
		func(idx int, src dao.PublishedArticle) domain.Article {
			return c.toDomain(dao.Article(src))
		}), nil
}
func (c CachedArticleRepository) Create(ctx context.Context, art domain.Article) (int64, error) {
	id, err := c.dao.Insert(ctx, c.toEntity(art))
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (c CachedArticleRepository) Update(ctx context.Context, art domain.Article) error {
	err := c.dao.UpdateById(ctx, c.toEntity(art))
	if err != nil {
		return err
	}
	return nil
}

func (c CachedArticleRepository) Sync(ctx context.Context, art domain.Article) (int64, error) {
	id, err := c.dao.Sync(ctx, c.toEntity(art))
	if err != nil {
		return 0, err
	}
	return id, nil

}

func (c CachedArticleRepository) SyncStatus(ctx context.Context, uid int64, id int64, status domain.ArticleStatus) error {
	err := c.dao.SyncStatus(ctx, uid, id, status.ToUint8())
	//数据库同步成功后，应该设置缓存，但是同步状态后续访问的频率不会很高，并且存在并发问题，所以直接删除缓存
	//到后面有需要查询的时候才设置缓存
	if err == nil {
		er := c.cache.DelFirstPage(ctx, uid)
		if er != nil {
			//删除缓存出错也没说什么大不了，记录日志即可
			//只要保持数据库的数据是最准确的即可
		}
	}
	return err
}

func (c CachedArticleRepository) GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error) {
	//先去查缓存，没有再查数据库，并且设置缓存
	if offset == 0 && limit < 100 {
		art, err := c.cache.GetFirstPage(ctx, uid)
		if err == nil {
			return art, nil
		} else {
			//记录日志，查询缓存失败
		}
	}
	arts, err := c.dao.GetByAuthor(ctx, uid, offset, limit)
	if err != nil {
		return nil, err
	}
	//把dao.art切片转换成domain.art切片
	res := slice.Map[dao.Article, domain.Article](arts, func(idx int, src dao.Article) domain.Article {
		return c.toDomain(src)
	})
	//查询作者的所有文章后，很有可能要访问第一页的数据，所以可以异步进行设置缓存
	//如果把作者所有的文章都加载到数据库中，那么数据量太大了，所以只把第一页的数据加载的数据库中，
	//并且第一页的数据也是访问频率最高的
	go func() {
		//这里不用前面是ctx是因为是这里设置了一个会过期的ctx，当超时的时候，会取消操作
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if offset == 0 && limit < 100 {
			err := c.cache.SetFirstPage(ctx, uid, res)
			if err != nil {
				//回写缓存失败，进行监控
			}
		}
	}()
	//预加载，过期时间尽可能短，判断用户可能会访问第一篇文章的数据
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c.preCache(ctx, res)
	}()
	return res, nil
}

func (c CachedArticleRepository) GetById(ctx context.Context, id int64) (domain.Article, error) {
	//先查缓存，再去查数据库
	res, err := c.cache.Get(ctx, id)
	if err == nil {
		return res, err
	}
	art, err := c.dao.GetById(ctx, id)
	if err != nil {
		return domain.Article{}, err
	}
	res = c.toDomain(art)
	//走到这，说明是查询数据库得到的数据，可以把数据设置到缓存中去
	go func() {
		er := c.cache.Set(ctx, res)
		if er != nil {
			//设置缓存出错也没说什么大不了，记录日志即可
			logger.Error(er)
		}
	}()
	return res, nil
}

func (c CachedArticleRepository) GetPubById(ctx context.Context, id int64) (domain.Article, error) {
	res, err := c.cache.GetPub(ctx, id)
	if err == nil {
		return res, nil
	}
	art, err := c.dao.GetPubById(ctx, id)
	if err != nil {
		return domain.Article{}, err
	}
	res = c.toDomain(dao.Article(art))
	author, err := c.userRepo.FindById(ctx, art.AuthorId)
	if err != nil {
		return domain.Article{}, err
	}
	res.Author.Name = author.Nickname

	//异步设置缓存
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		er := c.cache.SetPub(ctx, res)
		if er != nil {
			//回写缓存失败，记录日志
		}
	}()
	return res, nil
}
func (c *CachedArticleRepository) toEntity(art domain.Article) dao.Article {
	return dao.Article{
		Id:       art.Id,
		Title:    art.Title,
		Content:  art.Content,
		AuthorId: art.Author.Id,
		Status:   art.Status.ToUint8(),
	}
}
func (c *CachedArticleRepository) toDomain(art dao.Article) domain.Article {
	return domain.Article{
		Id:      art.Id,
		Title:   art.Title,
		Content: art.Content,
		Author: domain.Author{
			Id: art.AuthorId,
		},
		Ctime:  time.UnixMilli(art.Ctime),
		Utime:  time.UnixMilli(art.Utime),
		Status: domain.ArticleStatus(art.Status),
	}
}

// 设置第一篇文章的缓存，不是第一页！！！
func (c *CachedArticleRepository) preCache(ctx context.Context, arts []domain.Article) {
	const size = 1024 * 1024
	if len(arts) > 0 && len(arts[0].Content) < size {
		err := c.cache.Set(ctx, arts[0])
		if err != nil {
			return
		}
	}
}
