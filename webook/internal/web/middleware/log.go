package middleware

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"time"
)

type LogMiddlewareBuilder struct {
	logFn         func(ctx context.Context, l AccessLog)
	allowReqBody  bool //是否携带请求体，因为有的请求体很长，可以让他不携带
	allowRespBody bool
} // 这两个初始化的时候不传，默认是false,可以通过方法来该变成true
func NewLogMiddlewareBuilder(logFN func(ctx context.Context, l AccessLog)) *LogMiddlewareBuilder {
	return &LogMiddlewareBuilder{
		logFn: logFN,
	}
}

type AccessLog struct {
	Path     string        `json:"path"`
	Method   string        `json:"method"`
	ReqBody  string        `json:"req_body"`
	Status   int           `json:"status"`
	RespBody string        `json:"resp_body"`
	Duration time.Duration `json:"duration"`
}
type responseWriter struct {
	gin.ResponseWriter
	al *AccessLog
}

func (l *LogMiddlewareBuilder) AllowReqBody() *LogMiddlewareBuilder {
	l.allowReqBody = true
	return l
}
func (l *LogMiddlewareBuilder) AllowRespBody() *LogMiddlewareBuilder {
	l.allowRespBody = true
	return l
}
func (l *LogMiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		if len(path) > 1024 {
			path = path[:1024]
			//防止URL过长
		}
		method := ctx.Request.Method
		al := AccessLog{
			Path:   path,
			Method: method,
		}
		if l.allowReqBody {
			//进入这的话，说明是可以允许携带请求体的
			body, _ := ctx.GetRawData()
			if len(body) > 2048 {
				al.ReqBody = string(body[:2048])
			} else {
				al.ReqBody = string(body)
			}
			//预处理完成之后，还需要放回去
			ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		start := time.Now()
		if l.allowRespBody {
			ctx.Writer = &responseWriter{
				ResponseWriter: ctx.Writer,
				al:             &al,
			}
			defer func() {
				al.Duration = time.Since(start)
				l.logFn(ctx, al)
			}()
		}
		ctx.Next()
	}
}
