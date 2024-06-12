package service

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/events/article"
	"xiaoweishu/webook/internal/repository"
	logger2 "xiaoweishu/webook/pkg/logger"
)

type ArticleService interface {
	Save(ctx context.Context, art domain.Article) (int64, error)
	Publish(ctx context.Context, art domain.Article) (int64, error)
	Withdraw(ctx context.Context, uid int64, id int64) error
	GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error)
	GetById(ctx context.Context, id int64) (domain.Article, error)
	GetPubById(ctx context.Context, id, uid int64) (domain.Article, error)
	ListPub(ctx context.Context, start time.Time, offset, limit int) ([]domain.Article, error)
	Like100(ctx *gin.Context, biz string) ([]domain.Like100, error)
	UpdateTop200Articles(ctx context.Context) error
}
type articleService struct {
	repo     repository.ArticleRepository
	producer article.Producer
	l        logger2.LoggerV1
}

func (a *articleService) UpdateTop200Articles(ctx context.Context) error {
	return a.repo.GetTopArticles(ctx, "article", 200)

}

func (a *articleService) Like100(ctx *gin.Context, biz string) ([]domain.Like100, error) {
	if biz != "article" {
		return []domain.Like100{}, fmt.Errorf("unsupported biz type")
	} //健壮性。一般都不会触发，因为在web层已经写死了
	return a.repo.Like100(ctx, "article")
}

func (a *articleService) ListPub(ctx context.Context, start time.Time, offset, limit int) ([]domain.Article, error) {
	return a.repo.ListPub(ctx, start, offset, limit)
}

func NewArticleService(repo repository.ArticleRepository,
	producer article.Producer, l logger2.LoggerV1) ArticleService {
	return &articleService{
		repo:     repo,
		producer: producer,
		l:        l,
	}
}

func (a *articleService) Save(ctx context.Context, art domain.Article) (int64, error) {
	art.Status = domain.ArticleStatusUnpublished
	//只是编辑文章，还没有到发表，所以状态设置成未发表
	//id>0,说明这是一篇老文章
	if art.Id > 0 {
		err := a.repo.Update(ctx, art)
		return art.Id, err
	} //新文章，直接创建，此时的创建只是存在于数据库中
	return a.repo.Create(ctx, art)

}

// Publish 也就是同步的意思，将制作库的东西同步到线上库中
func (a *articleService) Publish(ctx context.Context, art domain.Article) (int64, error) {
	art.Status = domain.ArticleStatusPublished
	return a.repo.Sync(ctx, art)

}

func (a *articleService) Withdraw(ctx context.Context, uid int64, id int64) error {
	//隐藏文章，直接状态改成不可见或私人即可
	return a.repo.SyncStatus(ctx, uid, id, domain.ArticleStatusPrivate)

}

func (a *articleService) GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]domain.Article, error) {
	return a.repo.GetByAuthor(ctx, uid, offset, limit)
}

func (a *articleService) GetById(ctx context.Context, id int64) (domain.Article, error) {
	return a.repo.GetById(ctx, id)
}

func (a *articleService) GetPubById(ctx context.Context, id, uid int64) (domain.Article, error) {
	art, err := a.repo.GetPubById(ctx, id)
	go func() {
		if err == nil {
			er := a.producer.ProduceReadEvent(article.ReadEvent{
				Aid: id,
				Uid: uid,
			})
			if er != nil {
				a.l.Error("发送ReadEvent 失败",
					logger2.Int64("aid", id),
					logger2.Int64("uid", uid),
					logger2.Error(err))
			}
		}
	}()
	if err != nil {
		return domain.Article{}, err
	}
	return art, nil
}
