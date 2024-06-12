package dao

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
	"xiaoweishu/webook/pkg/migrator"
)

type InteractiveDAO interface {
	IncrReadCnt(ctx context.Context, biz string, bizId int64) error
	InsertLikeInfo(ctx context.Context, biz string, id int64, uid int64) error
	DeleteLikeInfo(ctx context.Context, biz string, id int64, uid int64) error
	InsertCollectionBiz(ctx context.Context, cb UserCollectionBiz) error
	GetLikeInfo(ctx context.Context, biz string, id int64, uid int64) (UserLikeBiz, error)
	GetCollectInfo(ctx context.Context, biz string, id int64, uid int64) (UserCollectionBiz, error)
	Get(ctx context.Context, biz string, id int64) (Interactive, error)
	BatchIncrReadCnt(ctx context.Context, bizs []string, ids []int64) error
	GetByIds(ctx context.Context, biz string, ids []int64) ([]Interactive, error)
}
type GORMInteractiveDAO struct {
	db *gorm.DB
}

func (DAO GORMInteractiveDAO) GetByIds(ctx context.Context, biz string, ids []int64) ([]Interactive, error) {
	var res []Interactive
	err := DAO.db.WithContext(ctx).Where("biz=? AND biz_id IN ?", biz, ids).Find(&res).Error
	return res, err

}

//  单个增加阅读数的代码跟批量增加阅读数的代码大部分一样，可以做一个复用版本

func (DAO GORMInteractiveDAO) BatchIncrReadCnt(ctx context.Context, bizs []string, ids []int64) error {
	for i := 0; i < len(bizs); i++ {
		err := DAO.db.WithContext(ctx).Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"read_cnt": gorm.Expr("`read_cnt` +1"), //由gorm来为我们做自增1
				"utime":    time.Now().UnixMilli(),
			}),
		}).Create(&Interactive{
			Biz:     bizs[i],
			BizId:   ids[i],
			Ctime:   time.Now().UnixMilli(),
			Utime:   time.Now().UnixMilli(),
			ReadCnt: 1,
		}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// 复用单个增加阅读数的代码
func (DAO GORMInteractiveDAO) BatchIncrReadCntV1(ctx context.Context, bizs []string, ids []int64) error {
	return DAO.db.Transaction(func(tx *gorm.DB) error {
		txDAO := NewGORMInteractiveDAO(tx)
		for i := 0; i < len(bizs); i++ {
			err := txDAO.IncrReadCnt(ctx, bizs[i], ids[i])
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (DAO GORMInteractiveDAO) IncrReadCnt(ctx context.Context, biz string, bizId int64) error {
	//每篇文章都有自己的interactive交互表
	//这里也需要考虑 up insert的语义，
	//如果是insert,那么就把utime和ctime改成now，readcnt改成1
	now := time.Now().UnixMilli()
	return DAO.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]interface{}{
			"read_cnt": gorm.Expr("`read_cnt` +1"), //由gorm来为我们做自增1
			"utime":    now,
		}),
	}).Create(&Interactive{
		Biz:     biz,
		BizId:   bizId,
		Ctime:   now,
		Utime:   now,
		ReadCnt: 1,
	}).Error
}

func (DAO GORMInteractiveDAO) InsertLikeInfo(ctx context.Context, biz string, id int64, uid int64) error {
	//每次插入的时候都要考虑是不是up insert
	//如果之前就已经点赞了，那么再次点赞的时候只需要刷新就行了
	//需要更新两个表，一个是用户的点赞记录表，一个是文章的点赞数
	return DAO.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"utime":  time.Now().UnixMilli(),
				"status": 1,
			}),
		}).Create(&UserLikeBiz{
			Uid:    uid,
			Biz:    biz,
			BizId:  id,
			Status: 1,
			Utime:  time.Now().UnixMilli(),
			Ctime:  time.Now().UnixMilli(),
		}).Error
		if err != nil {
			return err
		}
		return tx.WithContext(ctx).Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"like_cnt": gorm.Expr("`like_cnt` +1"),
				"utime":    time.Now().UnixMilli(),
			}),
		}).Create(&Interactive{
			Biz:     biz,
			BizId:   id,
			Ctime:   time.Now().UnixMilli(),
			Utime:   time.Now().UnixMilli(),
			LikeCnt: 1,
		}).Error
	})
}

func (DAO GORMInteractiveDAO) DeleteLikeInfo(ctx context.Context, biz string, id int64, uid int64) error {
	now := time.Now().UnixMilli()
	//软删除，把status改成0，即可
	//软删除再处理用户返回操作的时候具有较高的性能，而且可以保留数据
	return DAO.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&UserLikeBiz{}).
			Where("uid=? and biz=? and biz_id=?", uid, biz, id).
			Updates(map[string]interface{}{
				"utime":  now,
				"status": 0,
			}).Error
		if err != nil {
			return err
		}
		return tx.Model(&Interactive{}).Where("biz=? and biz_id=?", biz, id).Updates(map[string]interface{}{
			"utime":    now,
			"like_cnt": gorm.Expr("`like_cnt` -1"),
		}).Error
	})
}

func (DAO GORMInteractiveDAO) InsertCollectionBiz(ctx context.Context, cb UserCollectionBiz) error {
	now := time.Now().UnixMilli()
	cb.Utime = now
	cb.Ctime = now
	return DAO.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"utime": now,
			}),
		}).Create(&cb).Error
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			DoUpdates: clause.Assignments(map[string]interface{}{
				"collect_cnt": gorm.Expr("`collect_cnt` +1"),
				"utime":       now,
			}),
		}).Create(&Interactive{
			Biz:        cb.Biz,
			BizId:      cb.BizId,
			Ctime:      now,
			Utime:      now,
			CollectCnt: 1,
		}).Error
	})
}

func (DAO GORMInteractiveDAO) GetLikeInfo(ctx context.Context, biz string, id int64, uid int64) (UserLikeBiz, error) {
	var res UserLikeBiz
	err := DAO.db.WithContext(ctx).Where("biz=? AND biz_id=? AND uid =? AND status =?", biz, id, uid, 1).First(&res).Error
	if err != nil {
		return UserLikeBiz{}, err
	}
	return res, nil
}

func (DAO GORMInteractiveDAO) GetCollectInfo(ctx context.Context, biz string, id int64, uid int64) (UserCollectionBiz, error) {
	var res UserCollectionBiz
	err := DAO.db.WithContext(ctx).Where("biz=? AND biz_id=? AND uid =? ", biz, id, uid).First(&res).Error
	if err != nil {
		return UserCollectionBiz{}, err
	}
	return res, nil
}

func (DAO GORMInteractiveDAO) Get(ctx context.Context, biz string, id int64) (Interactive, error) {
	var res Interactive
	err := DAO.db.WithContext(ctx).Where("biz_id = ? and biz = ?", id, biz).First(&res).Error
	if err != nil {
		return Interactive{}, err
	}
	return res, nil

}

func NewGORMInteractiveDAO(db *gorm.DB) InteractiveDAO {
	return &GORMInteractiveDAO{
		db: db,
	}
}

type UserLikeBiz struct {
	Id     int64  `gorm:"primaryKey,autoIncrement"`
	Uid    int64  `gorm:"uniqueIndex:uid_biz_type_id"`
	BizId  int64  `gorm:"uniqueIndex:uid_biz_type_id"`
	Biz    string `gorm:"type:varchar(128);uniqueIndex:uid_biz_type_id"`
	Status int
	Utime  int64
	Ctime  int64
}

type UserCollectionBiz struct {
	Id int64 `gorm:"primaryKey,autoIncrement"`
	// 这边还是保留了了唯一索引
	Uid   int64  `gorm:"uniqueIndex:uid_biz_type_id"`
	BizId int64  `gorm:"uniqueIndex:uid_biz_type_id"`
	Biz   string `gorm:"type:varchar(128);uniqueIndex:uid_biz_type_id"`
	// 收藏夹的ID
	// 收藏夹ID本身有索引
	Cid   int64 `gorm:"index"`
	Utime int64
	Ctime int64
}

type Interactive struct {
	Id int64 `gorm:"primaryKey,autoIncrement"`
	// <bizid, biz>
	BizId int64 `gorm:"uniqueIndex:biz_type_id"`
	// WHERE biz = ?
	Biz string `gorm:"type:varchar(128);uniqueIndex:biz_type_id"`

	ReadCnt    int64
	LikeCnt    int64
	CollectCnt int64
	Utime      int64
	Ctime      int64
}

// 因为interactive实现了Entity接口，所以interctive和Entity可以认为是等价的，在迁移的时候，可以用T来表示这张表
// 这就是泛型的好处，对于其他表，可以让他实现这个接口，
// 然后在初始化的时候指定是哪张表就行了
// 这样就不用每张表都写一样的代码，减少了代码的复用
func (i Interactive) ID() int64 {
	return i.Id
}

func (i Interactive) CompareTo(dst migrator.Entity) bool {
	val, ok := dst.(Interactive)
	if !ok {
		return false
	}
	return i == val
}
