package job

import (
	"context"
	rlock "github.com/gotomicro/redis-lock"
	"sync"
	"time"
	"xiaoweishu/webook/internal/service"
	logger2 "xiaoweishu/webook/pkg/logger"
)

type RankingJob struct {
	svc       service.RankingService
	l         logger2.LoggerV1
	timeout   time.Duration
	client    *rlock.Client
	key       string
	localLock *sync.Mutex
	lock      *rlock.Lock
	//随机生成的负载均衡
	load int32
}

func NewRankingJob(
	svc service.RankingService,
	l logger2.LoggerV1,
	timeout time.Duration,
	client *rlock.Client) *RankingJob {
	return &RankingJob{
		key:       "job:ranking",
		l:         l,
		client:    client,
		localLock: &sync.Mutex{},
		timeout:   timeout,
		svc:       svc,
	}

}

func (r *RankingJob) Name() string {
	return "ranking"
}

func (r *RankingJob) Run() error {
	r.localLock.Lock()
	lock := r.lock
	if lock == nil { //证明还没抢到锁，去抢锁
		//4秒内完成枪锁操作
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
		defer cancel()
		//获取一个分布式锁，锁的时间是r.timeout，后面是重试策略，需要在一秒钟内申请成功
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
			//续约锁
			er := lock.AutoRefresh(r.timeout/2, r.timeout) //每过r.timeout/2就申请续约，续约的时间是r.timeout
			if er != nil {
				//续约失败
				r.localLock.Lock()
				r.lock = nil
				r.localLock.Unlock()
				//这里必须要用本地锁锁起来，否则其他goroutine可能会该该改变lock状态，
			}
		}()
	}
	//走到这里时，已经拿到了锁
	//在r.timeout内完成事务逻辑
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.svc.TopN(ctx)
}

//暴露出一个释放分布式锁的方法用于释放分布式锁
//直到最后关机的时候才释放分布式锁，避免有多个实列执行热榜任务，为了节省资源，我们只需要一个实列执行热榜任务就行了
//避免出现一个实列刚计算完热榜，另一个实列就拿到锁立刻去计算热榜，此时计算出来的热榜肯定跟刚才的没有什么区别，造成了资源的浪费

// 在调用run函数处，顺便defer一下close函数
// 其实不执行也是可以的，但获得分布式锁的实列关机后，就不会续约成功，过一段时间之后就有别的节点拿到分布式锁继续进行任务
func (r *RankingJob) Close() error {
	//其他goroutine，也会改变r.lock的值，所以此时要用本地锁锁起来
	r.localLock.Lock()
	lock := r.lock //此处读取r.lock的值要锁起来，避免被其他进程操作，读出出错误的值
	r.localLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	//一秒内解锁，若超时，会继续进行解锁，但是会被标记为进程终止，且会释放相关资源，用同一个ctx子进程也会被终止
	return lock.Unlock(ctx)
}

//！！！这个跟mysql的不一样的，mysql的是有nexttime的，所以执行完了就可以释放
//而这个执行完就释放的话，会立刻会其他节点抢占，且根据设置的cron进行定时调度，
//避免出现一个实列刚计算完热榜，另一个实列就拿到锁立刻去计算热榜，此时计算出来的热榜肯定跟刚才的没有什么区别，造成了资源的浪费
//所以可以关机再进行释放，只针对这个任务而言，或者不释放，关机后，一段时间不续约，锁就会过期，别的节点仍然可以拿到这个任务
