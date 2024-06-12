package service

import (
	"context"
	"errors"
	"github.com/ecodeclub/ekit/queue"
	"github.com/ecodeclub/ekit/slice"
	"math"
	"time"
	intrv1 "xiaoweishu/webook/api/proto/gen/intr/v1"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository"
)

type RankingService interface {
	TopN(ctx context.Context) error
	GetTopN(ctx context.Context) ([]domain.Article, error) //这个是方便用于测试
}
type BatchRankingService struct {
	//用来取点赞数
	intrSvc intrv1.InteractiveServiceClient
	//用来查找文章
	artSvc    ArticleService
	batchSize int
	scoreFunc func(likeCnt int64, utime time.Time) float64 //计算分数的算法，utime是用于筛查数据的，七天以前的就不需要了
	n         int
	repo      repository.RankingRepository
}

func NewBatchRankingService(intrSvc intrv1.InteractiveServiceClient,
	artSvc ArticleService) RankingService {
	return &BatchRankingService{
		intrSvc:   intrSvc,
		artSvc:    artSvc,
		batchSize: 100,
		n:         100,
		scoreFunc: func(likeCnt int64, utime time.Time) float64 {
			// 时间
			duration := time.Since(utime).Seconds()
			return float64(likeCnt-1) / math.Pow(duration+2, 1.5)
		},
	}
}

func (b *BatchRankingService) TopN(ctx context.Context) error {
	arts, err := b.topN(ctx)
	if err != nil {
		return err
	}
	//放到缓存中去
	return b.repo.ReplaceTopN(ctx, arts)
}

// 通过redis缓存查找热榜
func (b *BatchRankingService) GetTopN(ctx context.Context) ([]domain.Article, error) {
	return b.repo.GetTopN(ctx)
}

func (b *BatchRankingService) topN(ctx context.Context) ([]domain.Article, error) {
	offset := 0
	start := time.Now()
	ddl := start.Add(-7 * 24 * time.Hour) //七天以前的数据就不需要了
	type Score struct {
		score float64
		art   domain.Article
	}
	//排序算法，用的小根堆
	topN := queue.NewConcurrentPriorityQueue[Score](b.n, func(src Score, dst Score) int {
		if src.score > dst.score {
			return 1
		} else if src.score == dst.score {
			return 0
		} else {
			return -1
		}
	})

	for {
		arts, err := b.artSvc.ListPub(ctx, start, offset, b.batchSize)
		if err != nil {
			return nil, err
		}
		//slice.map的核心仍然转换成另一个类型的切片
		ids := slice.Map(arts, func(idx int, art domain.Article) int64 {
			return art.Id
		})
		//取点赞数
		//要求取出来的数据是 map[art.id]domain.article
		intrResp, err := b.intrSvc.GetByIds(ctx, &intrv1.GetByIdsRequest{
			Biz: "article",
			Ids: ids,
		})
		if err != nil {
			return nil, err
		}
		intrMap := intrResp.Intrs
		for _, art := range arts {
			intr := intrMap[art.Id]
			score := b.scoreFunc(intr.LikeCnt, art.Utime)
			element := Score{
				score: score,
				art:   art,
			}
			err = topN.Enqueue(element) //新元素入栈
			if errors.Is(err, queue.ErrOutOfCapacity) {
				//说明此时堆已经满了
				//此时需要让对对顶元素出去
				minEle, _ := topN.Dequeue()
				if minEle.score < score {
					_ = topN.Enqueue(element) //新元素个更大，新元素入堆
				} else {
					_ = topN.Enqueue(minEle) // 老元素更大，把老元素放回去
				}

			}
		}
		//进行下一批偏移量的计算
		offset = offset + len(arts)
		//如果这一批没有到达batchsize，说明数据已经取完了，
		//或者这一批的最一个数据的更新时间已经大于七天，那么就不需要继续往下取了
		if len(arts) < b.batchSize || arts[len(arts)-1].Utime.Before(ddl) {
			break
		}
	}
	//可以用len函数，也可以直接用b.n ，因为创建topn优先队列时，就固定了容量N
	res := make([]domain.Article, topN.Len())
	for i := topN.Len() - 1; i >= 0; i++ {
		ele, _ := topN.Dequeue()
		res[i] = ele.art
	}
	//因为先出来的是小的元素，所以需要反转一下，最后得出的res就是最大的元素在前面，符合热榜的功能
	return res, nil
}
