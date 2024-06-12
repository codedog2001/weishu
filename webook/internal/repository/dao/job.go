package dao

import (
	"context"
	"gorm.io/gorm"
	"time"
)

type JobDAO interface {
	Preempt(ctx context.Context) (Job, error)
	Release(ctx context.Context, jid int64) error
	UpdateUtime(ctx context.Context, id int64) error
	UpdateNextTime(ctx context.Context, id int64, t time.Time) error
}
type Job struct {
	Id         int64  `gorm:"primaryKey,autoIncrement"`
	Name       string `gorm:"type:varchar(128);unique"`
	Executor   string
	Expression string
	Cfg        string
	// 状态来表达，是不是可以抢占，有没有被人抢占
	Status int

	Version int

	NextTime int64 `gorm:"index"`

	Utime int64
	Ctime int64
}
type GORMJobDAO struct {
	db *gorm.DB
}

func (dao *GORMJobDAO) Preempt(ctx context.Context) (Job, error) {
	db := dao.db.WithContext(ctx)
	for {
		var j Job
		now := time.Now().UnixMilli()
		err := db.Where("status= ? AND next_time < ?",
			jobStatusWaiting, now).First(&j).Error
		if err != nil {
			return j, err
		}
		//version存在的作用是实现了乐观锁，即更新数据之前要先判断数据是否被别人修改了
		//高并发的场景下，如果有多个进程同时达到这里，那么先修改version的就会拿到锁，
		//而后修改version的，会因为jid和jversion联合找不到对应的数据，所以会导致更新失败，即rowaffected==0
		res := db.WithContext(ctx).Model(&Job{}).Where("id=? AND version =?", j.Id, j.Version).
			Updates(map[string]any{
				"status":  jobStatusRunning,
				"version": j.Version + 1,
				"utime":   now,
			})
		if res.Error != nil {
			return Job{}, res.Error
		}
		if res.RowsAffected == 0 {
			//没抢到,在ctx没超时的情况下进行重试
			continue
		}
		return j, err
	}
}

// 释放锁
func (dao *GORMJobDAO) Release(ctx context.Context, jid int64) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id=?", jid).
		Updates(map[string]any{
			"status": jobStatusWaiting,
			"utime":  now,
		}).Error
}

// 续约，高层应该设计一些函数检查utime，utime时间太古老的，就要终止掉，可能进程已经死掉，不能让他继续持有锁
func (dao *GORMJobDAO) UpdateUtime(ctx context.Context, jid int64) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id=?", jid).
		Updates(map[string]any{
			"utime": now,
		}).Error
}

// 更新下次定时任务的调度时间
func (dao *GORMJobDAO) UpdateNextTime(ctx context.Context, id int64, t time.Time) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id=?", id).Updates(map[string]any{
		"utime": now,
		//netxtime是上层计算好了传下来的，这里只负责数据库操作
		"next_time": t.UnixMilli(),
	}).Error
}

func NewGORMJobDAO(db *gorm.DB) JobDAO {
	return &GORMJobDAO{
		db: db,
	}
}

const (
	// jobStatusWaiting 没人抢
	jobStatusWaiting = iota
	// jobStatusRunning 已经被人抢了
	jobStatusRunning
	// jobStatusPaused 不再需要调度了
	jobStatusPaused
)
