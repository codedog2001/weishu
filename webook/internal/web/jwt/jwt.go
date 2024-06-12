package jwt

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"net/http"
	"strings"
	"time"
)

var JWTKey = []byte("k6CswdUm77WKcbM68UQUuxVsHSpTCwgK")
var RCJWTKey = []byte("k6CswdUm77WKcbM68UQUuxVsHSpTCwgA")

type Handler interface {
	ClearToken(ctx *gin.Context) error
	ExtractToken(ctx *gin.Context) string
	SetLoginToken(ctx *gin.Context, uid int64) error
	SetJWTToken(ctx *gin.Context, uid int64, ssid string) error
	CheckSession(ctx *gin.Context, ssid string) error
}

type RedisJWTHandler struct {
	client       redis.Cmdable
	signMethod   jwt.SigningMethod
	rcExpiration time.Duration
}

func NewRedisJWTHandler(client redis.Cmdable) Handler {
	return &RedisJWTHandler{
		client:       client,
		signMethod:   jwt.SigningMethodHS512, //token加密的方法
		rcExpiration: time.Hour * 24 * 7,     //长token的过期时间
	}
}

func (r RedisJWTHandler) ClearToken(ctx *gin.Context) error {
	ctx.Header("x-jwt-token", "")
	ctx.Header("x-refresh-token", "")
	uc := ctx.MustGet("user").(UserClaims)
	//值并不重要，检查的时候，检查的的键是不是存在，而且设置了过期时间和refresh过期时间是一样的，
	//所以当过期时间到了后，再拿过期token来访问本身就是不合法的，到不了查询redis这一步，所以这个时候就可以把redis中的ssid给删除掉
	return r.client.Set(ctx, fmt.Sprintf("users:ssid:%s", uc.Ssid), " ", r.rcExpiration).Err()
}

func (r RedisJWTHandler) ExtractToken(ctx *gin.Context) string {
	authCode := ctx.GetHeader("Authorization")
	if authCode == "" {
		// 没登录，没有 token, Authorization 这个头部都没有
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return " "
	}
	segs := strings.Split(authCode, " ")
	if len(segs) != 2 {
		// 没登录，Authorization 中的内容是乱传的
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return " "
	}
	return segs[1]
}

func (r RedisJWTHandler) SetLoginToken(ctx *gin.Context, uid int64) error {
	//与session结合，用到ssid，当refreshtoken失效后 ，把他的ssid放到redis中，然后每次都查询redis，看看对面有没有用无效ssid来访问
	ssid := uuid.New().String()
	err := r.setRefreshToken(ctx, uid, ssid)
	if err != nil {
		return err
	}
	//到这里只是拿到了refreshtoken，要继续拿refreshtoken去换jwttoken
	err = r.SetJWTToken(ctx, uid, ssid)
	if err != nil {
		return err
	}
	return nil

}

func (r RedisJWTHandler) SetJWTToken(ctx *gin.Context, uid int64, ssid string) error {
	uc := UserClaims{
		Uid:       uid,
		UserAgent: ctx.GetHeader("User-Agent"),
		Ssid:      ssid,
		RegisteredClaims: jwt.RegisteredClaims{
			// 1 分钟过期
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 30)),
		},
	}
	token := jwt.NewWithClaims(r.signMethod, uc)
	tokenStr, err := token.SignedString(JWTKey)
	if err != nil {
		ctx.String(http.StatusOK, "系统错误")
		return err
	}
	ctx.Header("x-jwt-token", tokenStr)
	return nil
}

// 通过检查ssid是否在redis中，来判断refreshtoken是否有效，若在，说明已经失效了
func (r RedisJWTHandler) CheckSession(ctx *gin.Context, ssid string) error {
	cnt, err := r.client.Exists(ctx, fmt.Sprintf("users:ssid:%s", ssid)).Result()
	if err != nil {
		return err
	}
	if cnt > 0 {
		return errors.New("token无效")
	}
	return nil
}

func (r *RedisJWTHandler) setRefreshToken(ctx *gin.Context, uid int64, ssid string) error {
	rc := RefreshClaims{
		Uid:  uid,
		Ssid: ssid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(r.rcExpiration)),
		},
	}
	token := jwt.NewWithClaims(r.signMethod, rc)
	tokenStr, err := token.SignedString(RCJWTKey) //长token的加密盐值
	if err != nil {
		ctx.String(http.StatusOK, "系统错误")
		return err
	}
	ctx.Header("x-refresh-token", tokenStr)
	return nil
}

type RefreshClaims struct {
	jwt.RegisteredClaims //这个字段是包里面实现好的，直接组合起来使用就行了
	Uid                  int64
	Ssid                 string
}
type UserClaims struct {
	jwt.RegisteredClaims //这个字段是包里面实现好的，直接组合起来使用就行了
	Uid                  int64
	UserAgent            string
	Ssid                 string
}
