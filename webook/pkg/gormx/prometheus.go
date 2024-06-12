package gormx

import (
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
	"sync"
	"time"
)

type Callbacks struct {
	vector *prometheus.SummaryVec
}

func (c *Callbacks) Name() string {
	return "prometheus"
}

func (c *Callbacks) Initialize(db *gorm.DB) error {
	if err := c.registerCallbacks(db); err != nil {
		return err
	}
	return nil
}

func (c *Callbacks) registerCallbacks(db *gorm.DB) error {
	// Create callbacks
	if err := db.Callback().Create().Before("gorm:create").Register("prometheus_create_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register("prometheus_create_after", c.After("CREATE")); err != nil {
		return err
	}

	// Query callbacks
	if err := db.Callback().Query().Before("gorm:query").Register("prometheus_query_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register("prometheus_query_after", c.After("QUERY")); err != nil {
		return err
	}

	// Raw callbacks
	if err := db.Callback().Raw().Before("gorm:raw").Register("prometheus_raw_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("prometheus_raw_after", c.After("RAW")); err != nil {
		return err
	}

	// Update callbacks
	if err := db.Callback().Update().Before("gorm:update").Register("prometheus_update_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("prometheus_update_after", c.After("UPDATE")); err != nil {
		return err
	}

	// Delete callbacks
	if err := db.Callback().Delete().Before("gorm:delete").Register("prometheus_delete_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("prometheus_delete_after", c.After("DELETE")); err != nil {
		return err
	}

	// Row callbacks
	if err := db.Callback().Row().Before("gorm:row").Register("prometheus_row_before", c.Before()); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register("prometheus_row_after", c.After("ROW")); err != nil {
		return err
	}

	return nil
}

var (
	once     sync.Once
	globalCb *Callbacks
)

func NewCallbacks(opts prometheus.SummaryOpts) *Callbacks {
	once.Do(func() {
		vector := prometheus.NewSummaryVec(opts, []string{"type", "table"})
		prometheus.MustRegister(vector)
		globalCb = &Callbacks{vector: vector}
	})
	return globalCb
}

func (c *Callbacks) Before() func(db *gorm.DB) {
	return func(db *gorm.DB) {
		start := time.Now()
		db.Set("start_time", start)
	}
}

func (c *Callbacks) After(typ string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		val, _ := db.Get("start_time")
		start, ok := val.(time.Time)
		if ok {
			duration := time.Since(start).Milliseconds()
			c.vector.WithLabelValues(typ, db.Statement.Table).Observe(float64(duration))
		}
	}
}

//type Callbacks struct {
//	vector *prometheus.SummaryVec
//}
//
//func (c *Callbacks) Name() string {
//	return "prometheus"
//}
//
//// 在调用前记录开始时间，调用结束后计算时间，就能够知道整个操作花了多少时间
//// 注册他们的钩子函数，他会自动调用
//func (c *Callbacks) Initialize(db *gorm.DB) error {
//
//	err := db.Callback().Create().Before("*").
//		Register("prometheus_create_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Create().After("*").
//		Register("prometheus_create_after", c.After("CREATE"))
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Query().Before("*").
//		Register("prometheus_query_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Query().After("*").
//		Register("prometheus_query_after", c.After("QUERY"))
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Query().Before("*").
//		Register("prometheus_raw_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Raw().After("*").
//		Register("prometheus_raw_after", c.After("RAW"))
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Update().Before("*").
//		Register("prometheus_update_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Update().After("*").
//		Register("prometheus_update_after", c.After("UPDATE"))
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Delete().Before("*").
//		Register("prometheus_delete_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Update().After("*").
//		Register("prometheus_delete_after", c.After("DELETE"))
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Row().Before("*").
//		Register("prometheus_row_before", c.Before())
//	if err != nil {
//		return err
//	}
//
//	err = db.Callback().Update().After("*").
//		Register("prometheus_row_after", c.After("ROW"))
//	return err
//}
//
//func NewCallbacks(opts prometheus.SummaryOpts) *Callbacks {
//	vector := prometheus.NewSummaryVec(opts,
//		[]string{"type", "table"})
//	prometheus.MustRegister(vector)
//	return &Callbacks{
//		vector: vector,
//	}
//}
//
//func (c *Callbacks) Before() func(db *gorm.DB) {
//	return func(db *gorm.DB) {
//		start := time.Now()
//		db.Set("start_time", start)
//	}
//}
//
//func (c *Callbacks) After(typ string) func(db *gorm.DB) {
//	return func(db *gorm.DB) {
//		val, _ := db.Get("start_time")
//		start, ok := val.(time.Time)
//		if ok {
//			duration := time.Since(start).Milliseconds()
//			c.vector.WithLabelValues(typ, db.Statement.Table).
//				Observe(float64(duration))
//		}
//	}
//}
