package validator

import (
	"context"
	"github.com/ecodeclub/ekit/slice"
	_ "github.com/twitchyliquid64/golang-asm/src"
	"golang.org/x/sync/errgroup"
	_ "golang.org/x/text/language"
	"gorm.io/gorm"
	"time"
	"xiaoweishu/webook/pkg/logger"
	"xiaoweishu/webook/pkg/migrator"
	"xiaoweishu/webook/pkg/migrator/events"
)

// 先确定是全量校验还是增量校验，然后确定校验的逻辑，从哪边到哪边
type Validator[T migrator.Entity] struct {
	base      *gorm.DB
	target    *gorm.DB
	l         logger.LoggerV1
	producer  events.Producer
	direction string
	utime     int64
	batchSize int
	//<=0 时 就中断 >0 就进行睡眠
	sleepInterval time.Duration
	fromBase      func(ctx context.Context, offset int) (T, error)
}

// 用泛型的好处就是减少代码的复用，比如这个验证器
// 验证的逻辑都是一样的，只是需要初始化两个验证器，他们的SRC dst 是不一样的
func NewValidator[T migrator.Entity](base *gorm.DB,
	target *gorm.DB,
	direction string,
	l logger.LoggerV1,
	producer events.Producer,

) *Validator[T] {
	res := &Validator[T]{
		base:      base,
		target:    target,
		l:         l,
		producer:  producer,
		direction: direction,
		batchSize: 100,
	}
	//一开始先初始化成从源表进行全量校验
	res.fromBase = res.fullFromBase
	return res
}

// 使用泛型的话，可以进行链式调用来更改结构体中的属性和初始化
// 这三个都是相同的返回值，所以可以进行链式调用
func (v *Validator[T]) Full() *Validator[T] {
	v.fromBase = v.fullFromBase
	return v
}
func (v *Validator[T]) Incr() *Validator[T] {
	v.fromBase = v.incrFromBase
	return v
}
func (v *Validator[T]) Utime(time int64) *Validator[T] {
	v.utime = time
	return v
}
func (v *Validator[T]) SleepInterval(interval time.Duration) *Validator[T] {
	v.sleepInterval = interval
	return v
}
func (v *Validator[T]) Validate(ctx context.Context) error {
	//正向校验和反向校验是不影响，所以可以一起进行
	var eg errgroup.Group
	eg.Go(func() error {
		return v.validateBaseToTarget(ctx)
	})
	eg.Go(func() error {
		return v.validateTargetToBase(ctx)
	})
	return eg.Wait()
}

// 全量校验
func (v *Validator[T]) fullFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	err := v.base.WithContext(dbCtx).Order("id").Offset(offset).First(&src).Error
	return src, err
	//一条条取出来进行对比
}

// 根据utime来进行更新，选择新增加的数据进行校验
func (v *Validator[T]) incrFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	err := v.base.WithContext(dbCtx).
		Order("id").
		Offset(offset).First(&src).Error
	return src, err
}
func (v *Validator[T]) validateBaseToTarget(ctx context.Context) error {
	offset := 0
	for {
		//basr to target 时，需要考虑是全量校验还是增量校验
		src, err := v.fromBase(ctx, offset)
		if err == context.Canceled || err == context.DeadlineExceeded {
			//超时或者取消了
			return nil
		}
		if err == gorm.ErrRecordNotFound {
			//说明源数据已经取完了
			////但是增量校验是要考虑一直运行的
			//这里用sleepinterval来表示增量校验和全量校验，如果<=0 那就是全量校验
			//如果》0 ，那么就是增量校验，睡眠一段时间后重启
			if v.sleepInterval <= 0 {
				return nil
			}
			time.Sleep(v.sleepInterval)
			continue
		}
		if err != nil {
			v.l.Error("base=>target 查询base失败", logger.Error(err))
			//不处理，等到下一次校验这批数会更新
			offset++ //这里是数据是一条条取的
			continue
		}
		var DstTS T // 因为是一条条取的，所以不需要用切片来接收
		err = v.target.WithContext(ctx).Where("id=?", src.ID()).First(&DstTS).Error
		switch err {
		case gorm.ErrRecordNotFound:
			//target里没有
			//发消息到kafka来修复
			v.notify(src.ID(), events.InconsistentEventTypeTargetMissing)
		case nil:
			equal := src.CompareTo(DstTS)
			if !equal {
				//丢消息到kafka，找到了但是数据不一致，需要修复
				v.notify(src.ID(), events.InconsistentEventTypeNEQ)
			}
		default:
			//出现问题，记录日志，不用处理，
			//相信之后的校验会解决这个问题
			v.l.Error("base=>target 查询target失败",
				logger.Error(err),
				logger.Int64("id", src.ID()))
		}
		offset++
	}
}

// 反向校验
func (v *Validator[T]) validateTargetToBase(ctx context.Context) error {
	offset := 0
	for {
		var DstTS []T
		err := v.target.WithContext(ctx).Select("id").Order("id").
			Offset(offset).Limit(v.batchSize).Find(&DstTS).Error
		if err == context.DeadlineExceeded || err == context.Canceled {
			return nil //超时或取消 ，直接中断
		}
		if err == gorm.ErrRecordNotFound || len(DstTS) == 0 {
			//数据被取完了
			if v.sleepInterval <= 0 {
				return nil //那就直接中断
			}
			time.Sleep(v.sleepInterval)
			continue
		}
		if err != nil {
			v.l.Error("target=>base 查询target失败", logger.Error(err))
			//不处理，等到下一次校验这批数会更新
			offset += len(DstTS)
			continue //继续下一批次
		}
		var scrTS []T
		ids := slice.Map(DstTS, func(idx int, t T) int64 {
			return t.ID()
		})
		err = v.base.WithContext(ctx).Select("id").Where("id in ?", ids).Find(&scrTS).Error
		if err == gorm.ErrRecordNotFound || len(scrTS) == 0 {
			//表示base里一条对应的数据，那么就要开始修复了
			//如果只有个别找不到的话，是不会触发这个错误的
			v.notifyBaseMissing(DstTS)
			offset += len(DstTS)
			continue
		}
		if err != nil {
			//错误就记录日志，然后继续下一批
			//因为这些数据会在之后的校验中再次被校验
			v.l.Error("target=>base 查询base失败", logger.Error(err))
			offset += len(DstTS)
			continue
			//保守做法的话，一样是发消息给kafka，让他来进行修复
		}
		diff := slice.DiffSetFunc(DstTS, scrTS, func(src, dst T) bool {
			return src.ID() == dst.ID() //不相等的数据就会被放在diff中
		})
		v.notifyBaseMissing(diff)
		if len(DstTS) < v.batchSize {
			if v.sleepInterval <= 0 {
				return nil //那就直接中断
			}
			time.Sleep(v.sleepInterval)
			continue
		}
		//循环的最后一定要记得修改偏移量，继续下一批
		offset += len(DstTS)
	}
}

// 这是批量，而且指定了类型，basemising只有可能是basemissing
func (v *Validator[T]) notifyBaseMissing(ts []T) {
	for _, val := range ts {
		v.notify(val.ID(), events.InconsistentEventTypeTBaseMissing)
	}
}

// 这是一条条的，而且没有指定小心类型
func (v *Validator[T]) notify(id int64, typ string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := v.producer.ProduceInconsistentEvent(ctx, events.InconsistentEvent{
		ID:        id,
		Direction: v.direction,
		Type:      typ,
	})
	if err != nil {
		v.l.Error("向kafka发送不一致消息失败",
			logger.Error(err),
			logger.Int64("id", id),
			logger.String("type", typ))
	}
}
