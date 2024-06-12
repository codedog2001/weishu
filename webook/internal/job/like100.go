package job

import (
	"context"
	rlock "github.com/gotomicro/redis-lock"
	"sync"
	"time"
	"xiaoweishu/webook/internal/service"
	logger2 "xiaoweishu/webook/pkg/logger"
)

type UpdateLikeJob struct {
	svc       service.ArticleService
	l         logger2.LoggerV1
	timeout   time.Duration
	client    *rlock.Client
	key       string
	localLock *sync.Mutex
	lock      *rlock.Lock
	load      int32
}

func NewUpdateRankingJob(
	svc service.ArticleService,
	l logger2.LoggerV1,
	timeout time.Duration,
	client *rlock.Client) *UpdateLikeJob {
	return &UpdateLikeJob{
		key:       "job:update_ranking",
		l:         l,
		client:    client,
		localLock: &sync.Mutex{},
		timeout:   timeout,
		svc:       svc,
	}
}

func (r *UpdateLikeJob) Name() string {
	return "update_ranking"
}

func (r *UpdateLikeJob) Run() error {
	r.localLock.Lock()
	lock := r.lock
	if lock == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
		defer cancel()

		lock, err := r.client.Lock(ctx, r.key, r.timeout, &rlock.FixIntervalRetry{
			Interval: time.Millisecond * 100,
			Max:      3}, time.Second)

		if err != nil {
			r.localLock.Unlock()
			r.l.Warn("获取分布式锁失败", logger2.Error(err))
			return nil
		}
		r.lock = lock
		r.localLock.Unlock()

		go func() {
			er := lock.AutoRefresh(r.timeout/2, r.timeout)
			if er != nil {
				r.localLock.Lock()
				r.lock = nil
				r.localLock.Unlock()
			}
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.svc.UpdateTop200Articles(ctx)
}

func (r *UpdateLikeJob) Close() error {
	r.localLock.Lock()
	lock := r.lock
	r.localLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return lock.Unlock(ctx)
}
