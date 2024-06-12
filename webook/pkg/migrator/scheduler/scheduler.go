package scheduler

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sync"
	"time"
	"xiaoweishu/webook/pkg/ginx"
	"xiaoweishu/webook/pkg/gormx/connpool"
	"xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/migrator"
	"xiaoweishu/webook/pkg/migrator/events"
	"xiaoweishu/webook/pkg/migrator/validator"
)

//Base 和 Target：
//
//这些术语在修复或故障恢复过程中可能互换角色，以保证系统的高可用性。
//它们提供了一种灵活的机制，允许系统在某一数据库出现故障时迅速切换到备用数据库。
//Src 和 Dst：
//
//在双写过程中，这两个术语的角色是固定的，src 始终是源数据库，而 dst 始终是目标数据库。
//固定角色确保了双写操作的确定性和一致性，避免因角色互换导致的数据混乱。
// Scheduler 用来统一管理整个迁移过程
//不是必须的

// full/incr是为了区分全量/增量
// srcfirst/dstfirst是为了更改base和target方向
// src和dst在一开始就是确定的，只有base和target会改变
type Scheduler[T migrator.Entity] struct {
	lock       sync.Mutex
	src        *gorm.DB
	dst        *gorm.DB
	l          logger.LoggerV1
	pattern    string
	cancelFull func()
	cancelIncr func()
	producer   events.Producer
	fulls      map[string]func()
	pool       *connpool.DoubleWritePool
}

// 在NewScheduler的时候，就要指定迁移的哪张表
func NewScheduler[T migrator.Entity](
	l logger.LoggerV1,
	src *gorm.DB,
	dst *gorm.DB,
	producer events.Producer,
	pool *connpool.DoubleWritePool,
) *Scheduler[T] {
	return &Scheduler[T]{
		src:      src,
		dst:      dst,
		l:        l,
		pattern:  connpool.PatternSrcOnly, //初始化的时候默认只读源数据库
		producer: producer,
		pool:     pool,
		cancelIncr: func() {
			//什么也也不用做

		},
		cancelFull: func() {
			//什么也不用做
		},
	}
}

// 传入一个分组名
func (s *Scheduler[T]) RegisterRoutes(server *gin.RouterGroup) {
	// 将这个暴露为 HTTP 接口
	// 你可以配上对应的 UI
	server.POST("/src_only", ginx.Wrap(s.SrcOnly))
	server.POST("/src_first", ginx.Wrap(s.SrcFirst))
	server.POST("/dst_first", ginx.Wrap(s.DstFirst))
	server.POST("/dst_only", ginx.Wrap(s.DstOnly))
	server.POST("/full/start", ginx.Wrap(s.StartFullValidation))
	server.POST("/full/stop", ginx.Wrap(s.StopFullValidation))
	server.POST("/incr/stop", ginx.Wrap(s.StopIncrementValidation))
	server.POST("/incr/start", ginx.WrapBody[StartIncrRequest](s.StartIncrementValidation))
}
func (s *Scheduler[T]) SrcFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcFirst //更改调度器的模式
	err := s.pool.UpdatePattern(connpool.PatternSrcFirst)
	if err != nil {
		return ginx.Result{}, err
	} //更改双写池的模式
	return ginx.Result{
		Msg: "OK",
	}, nil
}
func (s *Scheduler[T]) SrcOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcOnly
	err := s.pool.UpdatePattern(connpool.PatternSrcOnly)
	if err != nil {
		return ginx.Result{}, err
	}
	return ginx.Result{
		Msg: "OK",
	}, nil
}
func (s *Scheduler[T]) DstFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	//调度器和双写池的模式都需要更改
	s.pattern = connpool.PatternDstFirst
	err := s.pool.UpdatePattern(connpool.PatternDstFirst)
	if err != nil {
		return ginx.Result{}, err
	}
	return ginx.Result{
		Msg: "OK",
	}, nil
}
func (s *Scheduler[T]) DstOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternDstOnly
	err := s.pool.UpdatePattern(connpool.PatternDstOnly)
	if err != nil {
		return ginx.Result{}, err
	}
	return ginx.Result{
		Msg: "OK",
	}, nil
}

type StartIncrRequest struct {
	Utime int64 `json:"utime"`
	// 毫秒数
	// json 不能正确处理 time.Duration 类型
	Interval int64 `json:"interval"`
	//interval用来表示是增量校验
	//后面全量校验和增量校验中，若是增量校验，校验完了就不用中断，睡眠一段时间后继续执行
}

func (s *Scheduler[T]) newValidator() (*validator.Validator[T], error) {
	switch s.pattern {
	case connpool.PatternSrcOnly, connpool.PatternSrcFirst:
		return validator.NewValidator[T](s.src, s.dst, "SRC", s.l, s.producer), nil
	case connpool.PatternDstFirst, connpool.PatternDstOnly:
		return validator.NewValidator[T](s.dst, s.src, "DST", s.l, s.producer), nil
	default:
		return nil, fmt.Errorf("未知的 pattern %s", s.pattern)
	}
}

func (s *Scheduler[T]) StartIncrementValidation(c *gin.Context, req StartIncrRequest) (ginx.Result, error) {
	//开启增量校验
	s.lock.Lock()
	defer s.lock.Unlock()
	cancel := s.cancelIncr
	//newValidator()会跟s.pattern建立起合适的校验器
	//在新建校验器之前，就会运行full/incr来更改对应的校验模式
	v, err := s.newValidator() //新建一个校验器
	if err != nil {
		return ginx.Result{
			Code: 5,
			Msg:  "系统异常",
		}, err
	}
	//初始化参数，把v.frombase 改成incrfrombase
	v.Incr().Utime(req.Utime).SleepInterval(time.Duration(req.Interval) * time.Millisecond)
	go func() {
		var ctx context.Context
		//记录新的取消方法
		ctx, s.cancelIncr = context.WithCancel(context.Background())
		cancel()               //执行上一次的取消方法，取消上一次的增量校验
		err := v.Validate(ctx) //进行校验，如果遇到错误
		//遇到错误或者执行完增量校验或者被取消都会走到下面
		s.l.Warn("退出增量校验", logger.Error(err))
	}()
	return ginx.Result{
		Msg: "启动增量校验成功",
	}, nil
}
func (s *Scheduler[T]) StopIncrementValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelIncr() //取消增量校验
	return ginx.Result{
		Msg: "停止增量校验成功",
	}, nil
}
func (s *Scheduler[T]) StartFullValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	//cancel := s.cancelFull
	v, err := s.newValidator() //新建一个校验器
	if err != nil {
		return ginx.Result{}, nil
	}
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	//建立好本次校验的取消函数
	go func() {
		//cancel() //先取消上一次的全量校验
		err := v.Full().Validate(ctx)
		s.l.Warn("退出全量校验", logger.Error(err))
	}()
	return ginx.Result{
		Msg: "启动全量校验成功",
	}, nil
}
func (s *Scheduler[T]) StopFullValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelFull()
	return ginx.Result{
		Msg: "停止全量校验成功",
	}, nil
}
