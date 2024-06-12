package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository"
	repomocks "xiaoweishu/webook/internal/repository/mocks"
)

func Test_userService_Login(t *testing.T) {
	testCaes := []struct {
		name string
		mock func(ctrl *gomock.Controller) repository.UserRepository
		//预期输入
		ctx      context.Context
		email    string
		password string
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "登陆成功",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				//这个return只是模拟底层返回的数据和出错误，这样login函数才能继续往下走
				repo.EXPECT().FindByEmail(gomock.Any(), "123@qq.com").Return(domain.User{
					Email: "123@qq.com",
					//这是从数据库查询出来的密码，应该是加密过后的正确密码
					Password: "$2a$10$6QljaNXQp9rFwPA1QyTm7OIqULUKy6AdXC.D8J4GVjYadd86xezwS",
					Phone:    "17376913117",
				}, nil)
				return repo
			},
			//写测试用列
			email:    "123@qq.com",
			password: "123456#hello",
			wantUser: domain.User{
				Email: "123@qq.com",
				//想要登录成功，所以这里的密码，应该和底层数据库上返回的密码是一致的
				Password: "$2a$10$6QljaNXQp9rFwPA1QyTm7OIqULUKy6AdXC.D8J4GVjYadd86xezwS",
				Phone:    "17376913117",
			},
		},
		{
			name: "用户未找到",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				//这个return只是模拟底层返回的数据和出错误，这样login函数才能继续往下走
				repo.EXPECT().FindByEmail(gomock.Any(), "123@qq.com").Return(domain.User{}, repository.ErrUserNotFound) //这个error是模拟repository层返回的错误
				return repo
			},
			//写测试用列
			email: "123@qq.com",
			//用户输入的密码是未加密的
			password: "123456#hello",
			wantUser: domain.User{},
			wantErr:  ErrInvalidUserOrPassword, //这里的err是只service层返回的错误，跟repository层返回的错误不一样
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				//这个return只是模拟底层返回的数据和出错误，这样login函数才能继续往下走
				repo.EXPECT().FindByEmail(gomock.Any(), "123@qq.com").Return(domain.User{}, errors.New("随便一个错误"))
				return repo
			},
			//写测试用列
			email:    "123@qq.com",
			password: "123456#hello",
			wantUser: domain.User{},
			wantErr:  errors.New("随便一个错误"),
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				//这个return只是模拟底层返回的数据和出错误，这样login函数才能继续往下走
				repo.EXPECT().FindByEmail(gomock.Any(), "123@qq.com").Return(domain.User{}, errors.New("随便一个错误"))
				return repo
			},
			//写测试用列
			email:    "123@qq.com",
			password: "123456#hello",
			wantUser: domain.User{},
			wantErr:  errors.New("随便一个错误"),
		},
		//密码错误是发生在service层面，下层是没有发生错误的，已经把密码给出来了，是在service比较的时候出错的
		{
			name: "密码不对",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := repomocks.NewMockUserRepository(ctrl)
				//这个return只是模拟底层返回的数据和出错误，这样login函数才能继续往下走
				repo.EXPECT().FindByEmail(gomock.Any(), "123@qq.com").Return(domain.User{
					Email: "123@qq.com",
					//这是从数据库查询出来的密码，应该是加密过后的正确密码
					Password: "$2a$10$6QljaNXQp9rFwPA1QyTm7OIqULUKy6AdXC.D8J4GVjYadd86xezwS",
					Phone:    "17376913117",
				}, nil)
				return repo
			},
			//写测试用列
			email:    "123@qq.com",
			password: "123456#helo",
			wantErr:  ErrInvalidUserOrPassword,
		},
	}

	for _, tc := range testCaes {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			//利用mock构建依赖的对象
			repo := tc.mock(ctrl)
			svc := NewUserService(repo)
			user, err := svc.Login(tc.ctx, tc.email, tc.password)
			assert.Equal(t, tc.wantUser, user)
			assert.Equal(t, tc.wantErr, err)
		})
	}

}
func TestPasswordEncrypt(t *testing.T) {
	password := []byte("123456#hello")
	encrypted, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	assert.NoError(t, err)
	println(string(encrypted))
	err = bcrypt.CompareHashAndPassword(encrypted, []byte("123456#hello"))
	assert.NoError(t, err)
}
