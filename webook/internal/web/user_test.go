package web

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"testing"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/service"
	svcmocks "xiaoweishu/webook/internal/service/mocks"
)

// 每次都可以先把模板先写出来
//
//	func TestUserHandLer_Login(t *testing.T) {
//		testCases := []struct {
//			name string
//		}{}
//
//		for _, tc := range testCases {
//			t.Run(tc.name, func(t *testing.T) {
//
//			})
//		}
//	}
func TestUserHandLer_SignUp(t *testing.T) {
	testCases := []struct {
		name string
		//mock
		mock func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice)
		//构造请求，预期中的三个月输入
		reqBuilder func(t *testing.T) *http.Request
		//预期中的输出
		wantCode int
		wantBody string
	}{
		{
			name: "注册成功",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				userSvc := svcmocks.NewMockUserService(ctrl)
				//第二个预期输入可以写可以不写，想写就写，做一次提前预期检查
				userSvc.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(nil)
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				//signup中是没有用到codeavc的，所以不需要预期输入
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup",
					bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello#world123",
"confirmPassword": "hello#world123"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},
			wantCode: http.StatusOK,
			wantBody: "注册成功",
		},
		{
			name: "Bind出错",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				//bind出错是走不到usersvc和codesvc的所以不用写其对应的expect
				userSvc := svcmocks.NewMockUserService(ctrl)
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				//signup中是没有用到codeavc的，所以不需要预期输入
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup",
					bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello#world123",
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "邮箱格式不对",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				//邮箱格式不对一样是走不到svc的signup的
				userSvc := svcmocks.NewMockUserService(ctrl)
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				//signup中是没有用到codeavc的，所以不需要预期输入
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup",
					bytes.NewReader([]byte(`{
"email": "123@",
"password": "hello#world123",
"confirmPassword": "hello#world123"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},
			wantCode: http.StatusOK,
			wantBody: "你的邮箱格式不对",
		},
		{
			name: "两次输入的密码不同",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				userSvc := svcmocks.NewMockUserService(ctrl)
				//第二个预期输入可以写可以不写，想写就写，做一次提前预期检查
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				//signup中是没有用到codeavc的，所以不需要预期输入
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup",
					bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello#world123",
"confirmPassword": "helo#world123"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},
			wantCode: http.StatusOK,
			wantBody: "两次输入的密码不一致",
		},
		{
			name: "密码格式不对",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				userSvc := svcmocks.NewMockUserService(ctrl)
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				//signup中是没有用到codeavc的，所以不需要预期输入
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup",
					bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello",
"confirmPassword": "hello"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},
			wantCode: http.StatusOK,
			wantBody: "你的密码必须大于8位，包含数字，特殊字符",
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				userSvc := svcmocks.NewMockUserService(ctrl)
				userSvc.EXPECT().SignUp(gomock.Any(), domain.User{
					Email:    "123@qq.com",
					Password: "hello#world123",
				}).Return(errors.New("db错误"))
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup", bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello#world123",
"confirmPassword": "hello#world123"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},

			wantCode: http.StatusOK,
			wantBody: "系统异常",
		},
		{
			name: "邮箱冲突",
			mock: func(ctrl *gomock.Controller) (service.UserService, service.CodeSerVice) {
				userSvc := svcmocks.NewMockUserService(ctrl)
				//expect 当调用的函数中参数满足其期待的条件，那么就会触发你指定的错误
				userSvc.EXPECT().SignUp(gomock.Any(), domain.User{
					Email:    "123@qq.com",
					Password: "hello#world123",
				}).Return(service.ErrUserDuplicateEmail)
				codeSvc := svcmocks.NewMockCodeSerVice(ctrl)
				return userSvc, codeSvc
			},
			reqBuilder: func(t *testing.T) *http.Request {
				req, err := http.NewRequest(http.MethodPost,
					"/users/signup", bytes.NewReader([]byte(`{
"email": "123@qq.com",
"password": "hello#world123",
"confirmPassword": "hello#world123"
}`)))
				req.Header.Set("Content-Type", "application/json")
				assert.NoError(t, err)
				return req
			},

			wantCode: http.StatusOK,
			wantBody: "邮箱冲突",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			//利用mock构造handler
			userSvc, codeSvc := tc.mock(ctrl)
			hdl := NewUserHandLer(userSvc, codeSvc)
			//准备服务器，注册路由
			server := gin.Default()
			hdl.RegisterUsersRoutes(server)
			//准备req和记录的recorder
			req := tc.reqBuilder(t)
			recorder := httptest.NewRecorder()
			//执行
			server.ServeHTTP(recorder, req)
			//断言结果，看一下输出结果是不是和预期的一致
			assert.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantBody, recorder.Body.String())
		})
	}
}

//func TestMock(t *testing.T) {
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//	usersvc := svcmocks.NewMockUserService(ctrl)
//	//这里是预期发生的事情，没有预期调用的话，后面调用signup就会出错，必须按要求调用
//	usersvc.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(errors.New("mock error"))
//	err := usersvc.SignUp(context.Background(), domain.User{
//		Email: "123@qq.com",
//	})
//	t.Log(err)
//}
