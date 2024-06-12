package connpool

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"gorm.io/gorm"
	"xiaoweishu/webook/pkg/logger"
)

var errUnknownPattern = errors.New("未知的双写模式")

// 实现了connpoolbeginer 和connpool接口
type DoubleWritePool struct {
	src     gorm.ConnPool
	dst     gorm.ConnPool
	pattern *atomicx.Value[string]
	l       logger.LoggerV1
}

func (d *DoubleWritePool) UpdatePattern(pattern string) error {
	// 不是合法的 pattern
	switch pattern {
	case PatternSrcOnly, PatternSrcFirst, PatternDstOnly, PatternDstFirst:
		d.pattern.Store(pattern)
		return nil
	default:
		return errUnknownPattern
	}
}
func (d *DoubleWritePool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	pattern := d.pattern.Load()
	switch pattern {
	case PatternSrcOnly:
		src, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		return &DoubleWriteTx{
			src:     src,
			l:       d.l,
			pattern: pattern,
		}, err
	case PatternSrcFirst:
		src, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		dst, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			//srcFirst时候，dst并不重要，失败了记录日志就行了，不需要影响进程进行
			d.l.Error("双写目标表开启事务失败", logger.Error(err))
		}
		return &DoubleWriteTx{
			src:     src,
			dst:     dst,
			l:       d.l,
			pattern: pattern,
		}, err
	case PatternDstOnly:
		dst, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		return &DoubleWriteTx{
			dst:     dst,
			l:       d.l,
			pattern: pattern,
		}, err
	case PatternDstFirst:
		dst, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		src, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			//srcFirst时候，dst并不重要，失败了记录日志就行了，不需要影响进程进行
			d.l.Error("双写源表开启事务失败", logger.Error(err))
		}
		return &DoubleWriteTx{
			src:     src,
			dst:     dst,
			l:       d.l,
			pattern: pattern,
		}, nil
	default:
		return nil, errUnknownPattern
	}
}
func (d *DoubleWritePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("双写模式不支持这种模式")
	//功能：用于预编译 SQL 语句。
	//实现：由于双写模式下无法同时返回两个数据库的 sql.Stmt，所以此方法直接 panic，表示不支持双写模式下的预编译 SQL 语句。
}

// 会把grom生成的sql语句，在这里分配执行
func (d *DoubleWritePool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	//执行一条sql语句，可能带有参数，并且返回执行结果
	//ExecContext：用于执行不返回结果集的 SQL 语句，比如 INSERT、UPDATE、DELETE 等。
	//返回 sql.Result，包含影响的行数和最后插入的 ID
	switch d.pattern.Load() {
	case PatternSrcOnly:
		return d.src.ExecContext(ctx, query, args...) //只在源数据库执行语句
	case PatternSrcFirst:
		res, err := d.src.ExecContext(ctx, query, args...)
		if err == nil {
			_, err1 := d.dst.ExecContext(ctx, query, args...)
			if err1 != nil {
				d.l.Error("双写写入dst失败", logger.Error(err1))
				logger.String("sql", query)
			}
		}
		return res, err //只关心src执行结果
	case PatternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case PatternDstFirst:
		res, err := d.dst.ExecContext(ctx, query, args...)
		if err == nil {
			_, err1 := d.src.ExecContext(ctx, query, args...)
			if err1 != nil {
				d.l.Error("双写写入src失败", logger.Error(err1))
				logger.String("sql", query)
			}
		}
		return res, err
	default:
		return nil, errUnknownPattern
	}
}

// 用于执行返回多行结果集的 SQL 查询，比如 SELECT。返回 *sql.Rows，可以遍历结果集。
func (d *DoubleWritePool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	//查询比较简单，不需要只用操作源表
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		return nil, errUnknownPattern
	}
}

func (d *DoubleWritePool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		// 这样你没有带上错误信息
		//return &sql.Row{}
		//因为row里面err是私有的，无法更改
		panic(errUnknownPattern)
	}
}

const (
	PatternSrcOnly  = "src_only"
	PatternSrcFirst = "src_first"
	PatternDstFirst = "dst_first"
	PatternDstOnly  = "dst_only"
)

func NewDoubleWritePool(src *gorm.DB,
	dst *gorm.DB, l logger.LoggerV1) *DoubleWritePool {
	return &DoubleWritePool{
		src:     src.ConnPool,
		dst:     dst.ConnPool,
		pattern: atomicx.NewValueOf(PatternSrcOnly), //这是什么函数
		l:       l,
	}
}

// 装饰器模式
// 实现了gorm.ConnPool接口
// 实现了txcommiter接口
type DoubleWriteTx struct {
	src     *sql.Tx
	dst     *sql.Tx
	pattern string
	l       logger.LoggerV1
}

func (d *DoubleWriteTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("双写模式不支持")
}

func (d *DoubleWriteTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	//执行一条sql语句，可能带有参数，并且返回执行结果
	//ExecContext：用于执行不返回结果集的 SQL 语句，比如 INSERT、UPDATE、DELETE 等。
	//返回 sql.Result，包含影响的行数和最后插入的 ID
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.ExecContext(ctx, query, args...) //只在源数据库执行语句
	case PatternSrcFirst:
		res, err := d.src.ExecContext(ctx, query, args...)
		if err == nil {
			_, err1 := d.dst.ExecContext(ctx, query, args...)
			if err1 != nil {
				d.l.Error("双写写入dst失败", logger.Error(err1))
				logger.String("sql", query)
			}
		}
		return res, err //只关心src执行结果
	case PatternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case PatternDstFirst:
		res, err := d.dst.ExecContext(ctx, query, args...)
		if err == nil {
			_, err1 := d.src.ExecContext(ctx, query, args...)
			if err1 != nil {
				d.l.Error("双写写入src失败", logger.Error(err1))
				logger.String("sql", query)
			}
		}
		return res, err
	default:
		return nil, errUnknownPattern
	}
}

func (d *DoubleWriteTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		return nil, errUnknownPattern
	}
}

func (d *DoubleWriteTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		// 这样你没有带上错误信息
		//return &sql.Row{}
		//因为row里面err是私有的，无法更改
		panic(errUnknownPattern)
	}
}

// 提交事务
func (d *DoubleWriteTx) Commit() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Commit()
	case PatternSrcFirst:
		err := d.src.Commit()
		if err != nil {
			return err
		}
		if d.dst != nil {
			//前面begin 的时候，若dst开启事务失败，那么这里就会为nil，那就没必要提交dst的事务
			err1 := d.dst.Commit()
			if err1 != nil {
				//只用记录日志即可，不用中断掉
				d.l.Error("目标表提交事务失败")
			}
		}
		return nil
	case PatternDstFirst:
		err := d.dst.Commit()
		if err != nil {
			return err
		}
		if d.src != nil {
			//前面begin 的时候，若src开启事务失败，那么这里就会为nil，那就没必要提交src的事务
			err1 := d.src.Commit()
			if err1 != nil {
				//只用记录日志即可，不用中断掉
				d.l.Error("源表提交事务失败")
			}
		}
		return nil
	case PatternDstOnly:
		return d.dst.Commit()
	default:
		return errUnknownPattern
	}
}

// 回滚
func (d *DoubleWriteTx) Rollback() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Rollback()
	case PatternSrcFirst:
		err := d.src.Rollback()
		if err != nil {
			return err
		}
		if d.dst != nil {
			err1 := d.dst.Rollback()
			if err1 != nil {
				//只用记录日志即可，不用中断掉
				d.l.Error("目标表回滚事务失败")
			}
		} //这个模式只用保证src是数据准确即可
		return nil
	case PatternDstFirst:
		err := d.dst.Rollback()
		if err != nil {
			return err
		}
		if d.src != nil {
			err1 := d.src.Rollback()
			if err1 != nil {
				//只用记录日志即可，不用中断掉
				d.l.Error("源表回滚事务失败")
			}
		}
		return nil
	case PatternDstOnly:
		return d.dst.Rollback()
	default:
		return errUnknownPattern
	}
}
