package web

import (
	"errors"
	"fmt"
	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/service"
	ijwt "xiaoweishu/webook/internal/web/jwt"
)

const biz = "login"

type UserHandLer struct {
	//svc 和codesvc需要传入的是接口，而不再是结构体
	//接口本身就是引用类型，不需要再加*，否则后面调用函数会出错

	svc         service.UserService //其实就是gorm,db
	codeSvc     service.CodeSerVice
	emailExp    *regexp.Regexp
	passwordExp *regexp.Regexp
	ijwt.Handler
}

func NewUserHandLer(svc service.UserService, codeSvc service.CodeSerVice, hdl ijwt.Handler) *UserHandLer {
	//如此传入的参数的实现了该接口的结构体指针，go语言会做隐式的类型转换

	const (
		emailRegexPattern    = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
		passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
	)
	emailExp := regexp.MustCompile(emailRegexPattern, regexp.None)
	passwordExp := regexp.MustCompile(passwordRegexPattern, regexp.None)
	return &UserHandLer{
		emailExp:    emailExp,
		passwordExp: passwordExp,
		svc:         svc,
		codeSvc:     codeSvc,
		Handler:     hdl,
	}
}

func (u *UserHandLer) RegisterUsersRoutes(server *gin.Engine) {
	//设置分组路由
	ug := server.Group("/users")
	ug.POST("/signup", u.SignUp)
	//ug.POST("/login", u.Login)
	ug.POST("/profile1", u.Profile1)
	ug.POST("/edit", u.Edit)
	ug.POST("/login", u.LoginJWT)
	ug.POST("/profile", u.ProfileJWT)
	ug.POST("/login_sms/code/send", u.SendLoginSMSCode)
	ug.POST("/login_sms", u.LoginSMS)
	ug.POST("/logout", u.LogoutJWT)
}
func (u *UserHandLer) SendLoginSMSCode(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	//在生产环境中这里应该使用正则表达式进行校验
	if req.Phone == "" {
		ctx.JSON(http.StatusOK, Result{
			Code: 4,
			Msg:  "输入有误",
		})
		return
	}
	err = u.codeSvc.Send(ctx, biz, req.Phone)
	switch err {
	case nil:
		ctx.JSON(http.StatusOK, Result{
			Msg: "发送成功",
		})
	case service.ErrCodeSendTooMany:
		ctx.JSON(http.StatusOK, Result{
			Msg: "发送太频繁，请稍后再试",
		})
	default:
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

}
func (u *UserHandLer) LoginSMS(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	if req.Code == "" {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "请输入验证码",
		})
		return
	}
	//这边也要进行各种校验，比如验证码登录时的手机号是否有效，跟发送验证码的手机号是不是同一个等等
	ok, err := u.codeSvc.Verify(ctx, biz, req.Phone, req.Code)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	if !ok {
		ctx.JSON(http.StatusOK, Result{
			Code: 4,
			Msg:  "输入有误",
		})
		return
	}
	user, err := u.svc.FindOrCrete(ctx, req.Phone)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	//这里需要一个uid，所以要从上面取出
	err = u.SetLoginToken(ctx, user.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Code: 4,
		Msg:  "验证码校验通过",
	})
}

func (u *UserHandLer) SignUp(ctx *gin.Context) {
	type SignUpReq struct {
		Email           string `json:"email"`
		ConfirmPassword string `json:"confirmPassword"`
		Password        string `json:"password"`
	}
	var req SignUpReq
	//binb方法会根据content -Type 来解析你的数据到req里面
	//解析错了，就会直接写回一个400错误
	if err := ctx.Bind(&req); err != nil {
		return
	}
	//const只能放编译期就能确定的东西，emailExp是运行才能确定的东西，是不能直接放到这里面的

	//在参数校验的时候，一般只有超时了才会出现err ，timeout

	ok, err := u.emailExp.MatchString(req.Email)
	if err != nil {
		//这里不能把详细的错误返回给前端，但是可以记录到日志中
		ctx.String(http.StatusOK, "系统错误")
		return
	}
	if !ok {
		ctx.String(http.StatusOK, "你的邮箱格式不对")
		return
	}
	if req.ConfirmPassword != req.Password {
		ctx.String(http.StatusOK, "两次输入的密码不一致")
		return
	}

	ok, err = u.passwordExp.MatchString(req.Password)
	if err != nil {
		//这里不能把详细的错误返回给前端，但是可以记录到日志中
		ctx.String(http.StatusOK, "系统错误")
		return
	}
	if !ok {
		ctx.String(http.StatusOK, "你的密码必须大于8位，包含数字，特殊字符")
		return
	}
	fmt.Println(req)
	err = u.svc.SignUp(ctx, domain.User{
		Email:    req.Email,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrUserDuplicateEmail) {
		ctx.String(http.StatusOK, "邮箱冲突")
		return
	}
	if err != nil {
		ctx.String(http.StatusOK, "系统异常")
		return
	}
	ctx.String(http.StatusOK, "注册成功")
	return
	//接下来就到了数据库操作

}

//	func (u UserHandLer) Login(ctx *gin.Context) {
//		type LoginReq struct {
//			Email    string `json:"email"`
//			Password string `json:"password"`
//		}
//		var req LoginReq
//		if err := ctx.Bind(&req); err != nil {
//			return
//		} //拿到参数之后，就要进入下一层进行其他的逻辑处理
//		err := u.svc.Login(ctx, req.Email, req.Password)
//		if errors.Is(err, service.ErrInvalidUserOrPassword) {
//			ctx.String(http.StatusOK, "用户名或密码不对")
//			return
//		}
//		if err != nil {
//			ctx.String(http.StatusOK, "系统错误")
//			return
//		}
//		ctx.String(http.StatusOK, "登录成功")
//		return
//	}
func (u UserHandLer) Edit(ctx *gin.Context) {

}
func (u UserHandLer) Profile1(ctx *gin.Context) {
	ctx.String(http.StatusOK, "这是你的profile")
}
func (u UserHandLer) ProfileJWT(ctx *gin.Context) {
	c, ok := ctx.Get("claims")
	//必然有claims
	if !ok {
		//奇怪的错误，监控住这里
		ctx.String(http.StatusOK, "系统错误")
		return
	}
	claims, ok := c.(*ijwt.UserClaims) //类型断言
	if !ok {
		ctx.String(http.StatusOK, "系统错误")
		return
	}
	ctx.String(http.StatusOK, "这是你的profile")
	println(claims.Uid)
	//这边补充p'r
}
func (h UserHandLer) LoginJWT(ctx *gin.Context) {
	type LoginReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var req LoginReq
	if err := ctx.Bind(&req); err != nil {
		return
	}
	u, err := h.svc.Login(ctx, req.Email, req.Password)
	switch {
	case err == nil:
		err := h.SetLoginToken(ctx, u.Id)
		if err != nil {
			ctx.JSON(http.StatusOK, Result{
				Code: 5,
				Msg:  "系统错误",
				Data: nil,
			})
			return
		}
		ctx.String(http.StatusOK, "登录成功")
	case errors.Is(err, service.ErrInvalidUserOrPassword):
		ctx.String(http.StatusOK, "用户名或者密码不对")
	default:
		ctx.String(http.StatusOK, "系统错误")
	}
}
func (h *UserHandLer) RefreshToken(ctx *gin.Context) {
	//约定，前端在Authorization里面岱山跟这个refresh_token
	tokenStr := h.ExtractToken(ctx)
	var rc ijwt.RefreshClaims
	token, err := jwt.ParseWithClaims(tokenStr, &rc, func(token *jwt.Token) (interface{}, error) {
		return ijwt.RCJWTKey, nil
	})
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if token == nil || !token.Valid {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	err = h.CheckSession(ctx, rc.Ssid)
	if err != nil {
		//下面写的函数是当cnt>0时，一样会返回错误
		//token无效，或者redis出现问题
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	err = h.SetJWTToken(ctx, rc.Uid, rc.Ssid)
	if err != nil {
		return
	}
}
func (h UserHandLer) LogoutJWT(ctx *gin.Context) {
	err := h.ClearToken(ctx)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Msg: "退出登陆成功",
	})

}
