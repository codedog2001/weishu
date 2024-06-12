package fixer

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"xiaoweishu/webook/pkg/migrator"
	"xiaoweishu/webook/pkg/migrator/events"
)

type OverrideFixer[T migrator.Entity] struct {
	base    *gorm.DB
	target  *gorm.DB
	columns []string
}

func NewOverrideFixer[T migrator.Entity](base *gorm.DB, target *gorm.DB) (*OverrideFixer[T], error) {
	row, err := base.Model(new(T)).Order("id").Rows()
	if err != nil {
		return nil, err
	} //为了获取列名
	columns, err := row.Columns()
	//使用gorm.migrator接口来直接获取列名
	//stmt := &gorm.Statement{DB: base}
	//stmt.Parse(new(T))
	//migrator := base.Migrator()
	//columns, err := migrator.ColumnTypes(stmt.Table)
	return &OverrideFixer[T]{
		base:    base,
		target:  target,
		columns: columns,
	}, nil
}

// 最粗暴的方式，指定不会错，不管是什么类型都可以这么进行修复
func (f *OverrideFixer[T]) Fix(ctx context.Context, id int64) error {
	var t T
	err := f.base.WithContext(ctx).Where("id=?", id).First(&t).Error
	switch err {
	case gorm.ErrRecordNotFound:
		//也就是说在源库找不到，但是target库找到了，任何时候都是要以base为准的  ，src 和 dst都是可以作为base的，注意区分这两个概念
		return f.target.WithContext(ctx).Where("id=?", id).Delete(&t).Error
	//删除
	case nil:
		//up sert 语义
		return f.target.WithContext(ctx).Clauses(clause.OnConflict{
			//发送冲突时候用t的数据来更新相应的列
			//clause.Assignment{}  clause.AssignmentColumns(f.columns)
			//注意区分这两个函数
			DoUpdates: clause.AssignmentColumns(f.columns),
		}).Create(&t).Error
	default:
		return err
	}
}

// 优雅一些的方式
func (f *OverrideFixer[T]) FixV1(evt events.InconsistentEvent) error {
	switch evt.Type {
	//不相等或者目标库缺失 ， upsert或删除
	case events.InconsistentEventTypeNEQ, events.InconsistentEventTypeTargetMissing:
		var t T
		err := f.base.Where("id=?", evt.ID).First(&t).Error
		switch err {
		case gorm.ErrRecordNotFound:
			return f.target.Where("id=?", evt.ID).Delete("id=?", evt.ID).Error
		case nil:
			return f.target.Clauses(clause.OnConflict{
				DoUpdates: clause.AssignmentColumns(f.columns),
			}).Create(&t).Error
		default:
			return err
		}
	case events.InconsistentEventTypeTBaseMissing:
		return f.target.Where("id=?", evt.ID).Delete("id=?", evt.ID).Error
	}
	return nil
}
