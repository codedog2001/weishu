package ioc

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
	"xiaoweishu/webook/internal/web"
	"xiaoweishu/webook/internal/web/jwt"
	"xiaoweishu/webook/internal/web/middleware"
	"xiaoweishu/webook/pkg/ginx/mididlewares/ratelimit"
	"xiaoweishu/webook/pkg/logger"
)

func InitWebServer(mdls []gin.HandlerFunc, userHdl *web.UserHandLer,
	oauth2WechatHdl *web.OAuth2WechatHandLer,
	artHdl *web.ArticleHandler) *gin.Engine {
	server := gin.Default()
	server.Use(mdls...)
	userHdl.RegisterUsersRoutes(server)
	oauth2WechatHdl.RegisterRoutes(server)
	artHdl.RegisterRoutes(server)
	return server
}

func InitGinMiddlewares(redisClient redis.Cmdable, hdl jwt.Handler, l logger.LoggerV1) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		cors.New(cors.Config{
			//AllowAllOrigins: true,
			//AllowOrigins:     []string{"http://localhost:3000"},
			AllowCredentials: true,

			AllowHeaders: []string{"Content-Type", "Authorization"},
			// 这个是允许前端访问你的后端响应中带的头部
			ExposeHeaders: []string{"x-jwt-token"},
			//AllowHeaders: []string{"content-type"},
			//AllowMethods: []string{"POST"},
			AllowOriginFunc: func(origin string) bool {
				if strings.HasPrefix(origin, "http://localhost") {
					//if strings.Contains(origin, "localhost") {
					return true
				}
				return strings.Contains(origin, "your_company.com")
			},
			MaxAge: 12 * time.Hour,
		}),
		func(ctx *gin.Context) {
			println("这是我的 Middleware")
		},
		ratelimit.NewBuilder(redisClient, time.Second, 1000).Build(),
		middleware.NewLogMiddlewareBuilder(func(ctx context.Context, al middleware.AccessLog) {
			l.Debug("", logger.Field{Key: "req", Val: al})
		}).AllowReqBody().AllowRespBody().Build(),
		//通过这个中间件，就可以把请求体和响应体打印到日志中去
		middleware.NewLoginJWTMiddlewareBuilder(hdl).CheckLogin(),
	}
}
