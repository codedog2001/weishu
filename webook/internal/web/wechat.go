package web

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"xiaoweishu/webook/internal/service"
	"xiaoweishu/webook/internal/service/oauth2/wechat"
	ijwt "xiaoweishu/webook/internal/web/jwt"
)

type OAuth2WechatHandLer struct {
	svc wechat.Service
	ijwt.Handler
	userSVc service.UserService
}

func NewOAuth2WechatHandler(svc wechat.Service, userSVc service.UserService, hdl ijwt.Handler) *OAuth2WechatHandLer {
	return &OAuth2WechatHandLer{
		svc:     svc,
		userSVc: userSVc,
		Handler: hdl,
	}
}

func (h *OAuth2WechatHandLer) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/oauth2/wechat")
	g.GET("/authurl", h.AuthURL)
	g.Any("/callback", h.Callback)
}
func (h *OAuth2WechatHandLer) AuthURL(ctx *gin.Context) {
	state := "  "
	val, err := h.svc.AuthURL(ctx, state)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "构造跳转URL失败",
			Code: 5,
		})
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Data: val,
	})

}
func (h *OAuth2WechatHandLer) Callback(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	info, err := h.svc.VerifyCode(ctx, code, state)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "验证失败",
			Code: 5,
		})
		return
	}
	//走到这，说明已经验证成功了，就要开始设置长短token
	//
	u, err := h.userSVc.FindOrCrateByWechat(ctx, info)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	err = h.SetLoginToken(ctx, u.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	} //需要uid，所以需要查表
	ctx.JSON(http.StatusOK, Result{
		Msg: "ok",
	})
}
