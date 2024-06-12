package job

import (
	"context"
	"fmt"
	"golang.org/x/sync/semaphore"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/service"
	logger2 "xiaoweishu/webook/pkg/logger"
)

//一个调度器job对应一个本地方法
//基于mysql实现的分布式任务调度器

type Executor interface {
	Name() string
	//Exec ctx 是全局控制，Executor的实现者注意要正确处理ctx超时或取消
	Exec(ctx context.Context, j domain.Job) error
}

// LocalFuncExecutor 调用本地方法的
type LocalFuncExecutor struct {
	funcs map[string]func(ctx context.Context, j domain.Job) error
}

func (l *LocalFuncExecutor) Name() string {
	return "local"
}

func (l *LocalFuncExecutor) Exec(ctx context.Context, j domain.Job) error {
	//根据调度器的名字取出对应的执行方法
	fn, ok := l.funcs[j.Name]
	if !ok {
		return fmt.Errorf("未注册本地方法%s", j.Name)
	}
	return fn(ctx, j) //执行该函数，并返回错误
}

type Scheduler struct {
	dbTimeout time.Duration
	svc       service.CronJobService
	l         logger2.LoggerV1
	limiter   *semaphore.Weighted
	executors map[string]Executor
}

func NewScheduler(svc service.CronJobService, l logger2.LoggerV1) *Scheduler {
	return &Scheduler{
		svc:       svc,
		l:         l,
		limiter:   semaphore.NewWeighted(100),
		executors: map[string]Executor{},
	}
}
func (l *LocalFuncExecutor) RegisterFunc(name string, fn func(ctx context.Context, j domain.Job) error) {
	l.funcs[name] = fn
}

func NewLocalFuncExecutor() *LocalFuncExecutor {
	return &LocalFuncExecutor{funcs: map[string]func(ctx context.Context, j domain.Job) error{}}
}
func (s *Scheduler) Schedule(ctx context.Context) error {
	//上面会传入一个带超时的ctx来进行调度
	//超市后就会退出for循环
	for {
		//放弃调度了
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := s.limiter.Acquire(ctx, 1)
		if err != nil {
			return err
		}
		//给下面的数据库操作设置了一个时间期限
		dbctx, cancel := context.WithTimeout(ctx, s.dbTimeout)
		j, err := s.svc.Preempt(dbctx)
		cancel() //数据库操作执行完之后直接cancel .及时释放资源
		if err != nil {
			//抢锁失败
			//直接下一轮抢锁，或者睡一段时间再抢
			continue
		}
		//此时已经拿到了锁，要开始调度执行j
		exec, ok := s.executors[j.Executor] //具体的执行器方法需要自己去实现，高度解耦
		if !ok {
			//直接中断，也可以下一轮
			s.l.Error("找不到执行器", logger2.Int64("jid", j.Id),
				logger2.String("executor", j.Executor))
			continue
		}

		go func() {
			defer func() {
				//限制最多一百个进程来抢分布式锁
				s.limiter.Release(1)
				j.CancelFunc() //执行任务完成，取消掉定时器，下一此调用会重新生成
			}()
			err1 := exec.Exec(ctx, j)
			if err1 != nil {
				s.l.Error("任务执行失败", logger2.Int64("jid", j.Id),
					logger2.Error(err1))
				return
			}
			//只有成功了，才会去设置下次调度的时间，调度的任务和当前的任务是一样的
			err1 = s.svc.ResetNextTime(ctx, j)
			if err1 != nil {
				s.l.Error("重置下次执行时间失败",
					logger2.Int64("jid", j.Id),
					logger2.Error(err1))
			}
		}()
	}
}
