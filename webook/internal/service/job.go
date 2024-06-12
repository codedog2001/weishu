package service

import (
	"context"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository"
	logger2 "xiaoweishu/webook/pkg/logger"
)

//定时器会在设定的时间触发任务，但在任务执行前，会先尝试抢占分布式锁。
//如果成功抢占到锁，那么任务就会顺利执行。如果抢不到锁，说明有其他实例或线程已经在执行该任务，
//此时当前实例或线程就会放弃执行，等待下一个调度周期再次尝试抢占锁。

type CronJobService interface {
	Preempt(ctx context.Context) (domain.Job, error) //抢占分布式锁
	//抢占分布式锁也算是job一部分，定时任务需要先去抢锁，再执行
	ResetNextTime(ctx context.Context, j domain.Job) error //设置下次定时任务调度的时间
	//Release(ctx context.Context, job domain.Job) error
	// 暴露 job 的增删改查方法
}
type cronJobService struct {
	repo            repository.CronJobRepository
	l               logger2.LoggerV1
	refreshInterval time.Duration
}

func (c *cronJobService) Preempt(ctx context.Context) (domain.Job, error) {
	j, err := c.repo.Preempt(ctx)
	if err != nil {
		return domain.Job{}, err
	}
	//到这肯定是已经拿到了锁，所以这里可以开启一个定时器，定时更细utime，相当于续约锁
	//只有当主动调用cancelfunc时才会释放锁
	ticker := time.NewTicker(c.refreshInterval)
	go func() {
		for range ticker.C {
			c.refresh(j.Id)
		}
	}()
	j.CancelFunc = func() {
		ticker.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := c.repo.Release(ctx, j.Id)
		if err != nil {
			c.l.Error("释放Job失败",
				logger2.Error(err),
				logger2.Int64("jid", j.Id))
		}
	}
	return j, err
}

func (c *cronJobService) ResetNextTime(ctx context.Context, j domain.Job) error {
	nextTime := j.NextTime()
	return c.repo.UpdateNextTime(ctx, j.Id, nextTime)
}
func (c *cronJobService) refresh(id int64) {
	//本质上就是更新时间
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := c.repo.UpdateUtime(ctx, id)
	if err != nil {
		c.l.Error("续约失败", logger2.Error(err),
			logger2.Int64("jid", id))
	}

}
func NewCronJobService(l logger2.LoggerV1, refreshInterval time.Duration, repo repository.CronJobRepository) CronJobService {
	return &cronJobService{
		repo:            repo,
		l:               l,
		refreshInterval: refreshInterval,
	}
}
