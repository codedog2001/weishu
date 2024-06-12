package prometheus2

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
	"strconv"
	"time"
)

// count只能统计一些只增不减的数，比如http请求的总数，错误数，成功数等
// gauge可以统计一些增加或者减少的量，比如内存，cpu等
// Summary主要用于描述指标的分布情况，例如请求响应时间。 也就是多维度的
// 通过SummaryVec，可以在同一个指标名称下使用不同的标签值创建多个度量，从而更全面地了解指标的分布情况。
type Callbacks struct {
	Namespace  string
	Subsystem  string
	Name       string
	InstanceID string
	Help       string
	vector     *prometheus.SummaryVec
}

func (c *Callbacks) Register(db *gorm.DB) error {
	vector := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:      c.Name,
			Subsystem: c.Subsystem,
			Namespace: c.Namespace,
			Help:      c.Help,
			ConstLabels: map[string]string{
				"db_name":     db.Name(),
				"instance_id": c.InstanceID,
			},
			Objectives: map[float64]float64{
				0.9:  0.01,
				0.99: 0.001,
			},
		},
		[]string{"type", "table"})
	prometheus.MustRegister(vector)
	c.vector = vector

	// Querys
	err := db.Callback().Query().Before("*").
		Register("prometheus_query_before", c.before("query"))
	if err != nil {
		return err
	}

	err = db.Callback().Query().After("*").
		Register("prometheus_query_after", c.after("query"))
	if err != nil {
		return err
	}

	err = db.Callback().Raw().Before("*").
		Register("prometheus_raw_before", c.before("raw"))
	if err != nil {
		return err
	}

	err = db.Callback().Query().After("*").
		Register("prometheus_raw_after", c.after("raw"))
	if err != nil {
		return err
	}

	err = db.Callback().Create().Before("*").
		Register("prometheus_create_before", c.before("create"))
	if err != nil {
		return err
	}

	err = db.Callback().Create().After("*").
		Register("prometheus_create_after", c.after("create"))
	if err != nil {
		return err
	}

	err = db.Callback().Update().Before("*").
		Register("prometheus_update_before", c.before("update"))
	if err != nil {
		return err
	}

	err = db.Callback().Update().After("*").
		Register("prometheus_update_after", c.after("update"))
	if err != nil {
		return err
	}

	err = db.Callback().Delete().Before("*").
		Register("prometheus_delete_before", c.before("delete"))
	if err != nil {
		return err
	}

	err = db.Callback().Delete().After("*").
		Register("prometheus_delete_after", c.after("delete"))
	if err != nil {
		return err
	}
	return nil
}

// before 这里就是为了保持风格统一
func (c *Callbacks) before(typ string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		start := time.Now()
		db.Set("start_time", start)
	}
}

func (c *Callbacks) after(typ string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		val, _ := db.Get("start_time")
		// 如果上面没找到，这边必然断言失败
		start, ok := val.(time.Time)
		if !ok {
			// 没必要记录，有系统问题，可以记录日志
			return
		}
		duration := time.Since(start)
		c.vector.WithLabelValues(typ, db.Statement.Table).
			Observe(float64(duration.Milliseconds()))
	}
}

type Builder struct {
	Namespace  string
	Subsystem  string
	Name       string
	InstanceId string
	Help       string
}

func (b *Builder) BuildResponseTime() gin.HandlerFunc {
	//pattern是指命中的路由
	labels := []string{"method", "path", "status"}
	vector := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: b.Namespace,
		Subsystem: b.Subsystem,
		Help:      b.Help,
		Name:      b.Name + "_rest_time",
		ConstLabels: map[string]string{
			"instance_id": b.InstanceId,
		},
		Objectives: map[float64]float64{
			0.5:   0.01,
			0.9:   0.01,
			0.99:  0.001,
			0.999: 0.0001,
		},
	}, labels)
	prometheus.MustRegister(vector)
	return func(ctx *gin.Context) {
		start := time.Now()
		defer func() {
			duration := time.Since(start).Milliseconds()
			method := ctx.Request.Method
			pattern := ctx.FullPath()
			status := ctx.Writer.Status()
			vector.WithLabelValues(method, pattern, strconv.Itoa(status)).Observe(float64(duration))
		}()
		ctx.Next() //先继续往下执行，最后在执行这个func，计算总的http相应时间
	}
}
func (b *Builder) BuildActiveRequest() gin.HandlerFunc {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: b.Namespace,
		Subsystem: b.Subsystem,
		Help:      b.Help,
		Name:      b.Name + "_active_req",
		ConstLabels: map[string]string{
			"instance_id": b.InstanceId,
		},
	})
	prometheus.MustRegister(gauge)
	return func(ctx *gin.Context) {
		gauge.Inc()
		defer gauge.Dec()
		ctx.Next()
	}

}
