package ioc

import (
	rlock "github.com/gotomicro/redis-lock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
	"time"
	"xiaoweishu/webook/internal/job"
	"xiaoweishu/webook/internal/service"
	"xiaoweishu/webook/pkg/logger"
)

func InitRankingJob(svc service.RankingService, client *rlock.Client, l logger.LoggerV1) *job.RankingJob {
	return job.NewRankingJob(svc, l, time.Second*30, client)
}
func InitLikeJob(svc service.ArticleService, client *rlock.Client, l logger.LoggerV1) *job.UpdateLikeJob {
	return job.NewUpdateRankingJob(svc, l, time.Second*30, client)
}

func InitJobs(l logger.LoggerV1, rjob *job.RankingJob, ljob *job.UpdateLikeJob) *cron.Cron {
	builder := job.NewCronJobBuilder(l, prometheus.SummaryOpts{
		Namespace: "zx",
		Subsystem: "webook",
		Name:      "cron_job",
		Help:      "定时任务执行",
		Objectives: map[float64]float64{
			0.5:   0.01,
			0.75:  0.01,
			0.9:   0.01,
			0.99:  0.001,
			0.999: 0.0001,
		},
	})
	//一秒执行一次
	expr := cron.New(cron.WithSeconds())
	_, err := expr.AddJob("@every 1s", builder.Build(rjob))
	if err != nil {
		panic(err)
	}
	_, err = expr.AddJob("@every 10m", builder.Build(ljob))
	if err != nil {
		panic(err)
	}
	return expr
}
