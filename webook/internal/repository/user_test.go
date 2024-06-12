package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"testing"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository/cache"
	cachemocks "xiaoweishu/webook/internal/repository/cache/mocks"
	"xiaoweishu/webook/internal/repository/dao"
	daomocks "xiaoweishu/webook/internal/repository/dao/mocks"
)

func TestCacheUserRepository_FindById(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	now := time.UnixMilli(nowMs)
	testCases := []struct {
		name string
		//利用mock模拟repository所依赖的dao层和cache层
		mock     func(ctrl *gomock.Controller) (cache.UserCache, dao.UserDAO)
		ctx      context.Context
		uid      int64
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "查找成功，但是缓存未命中",
			mock: func(ctrl *gomock.Controller) (cache.UserCache, dao.UserDAO) {
				uid := int64(1)
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				//每一次返回的对象是不一样的，要根据写的函数去返回对应的参数
				//在rediscache返回的是domainuser,在dao层返回的是dao.user，所以在不同的expect处要返回不同的值
				c.EXPECT().Get(gomock.Any(), uid).
					Return(domain.User{}, cache.ErrKeyNotExist)
				d.EXPECT().FindById(gomock.Any(), uid).
					Return(dao.User{
						Id: uid,
						Email: sql.NullString{
							String: "123@qq.com",
							Valid:  true,
						},
						Password: "123456",
						Phone: sql.NullString{
							String: "17376913117",
							Valid:  true,
						},
						Ctime: nowMs,
						Utime: 102,
					}, nil)
				c.EXPECT().Set(gomock.Any(), domain.User{
					Id:       uid,
					Email:    "123@qq.com",
					Password: "123456",
					Phone:    "17376913117",
					Ctime:    now,
				}).Return(nil)
				//即从数据中找到了，然后写入cache中
				return c, d
			},
			ctx: context.Background(),
			uid: 1,
			wantUser: domain.User{
				Id:       1,
				Email:    "123@qq.com",
				Password: "123456",
				Phone:    "17376913117",
				Ctime:    now,
			},
			wantErr: nil,
		},
		{
			name: "缓存命中",
			mock: func(ctrl *gomock.Controller) (cache.UserCache, dao.UserDAO) {
				uid := int64(1)
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				//每一次返回的对象是不一样的，要根据写的函数去返回对应的参数
				//在rediscache返回的是domainuser,在dao层返回的是dao.user，所以在不同的expect处要返回不同的值
				c.EXPECT().Get(gomock.Any(), uid).
					Return(domain.User{Id: 1,
						Email:    "123@qq.com",
						Password: "123456",
						Phone:    "17376913117",
						Ctime:    now,
					}, nil)
				//即从数据中找到了，然后写入cache中
				return c, d
			},
			ctx: context.Background(),
			uid: 1,
			wantUser: domain.User{
				Id:       1,
				Email:    "123@qq.com",
				Password: "123456",
				Phone:    "17376913117",
				Ctime:    now,
			},
			wantErr: nil,
		},
		{
			name: "未找到用户",
			mock: func(ctrl *gomock.Controller) (cache.UserCache, dao.UserDAO) {
				uid := int64(1)
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				//每一次返回的对象是不一样的，要根据写的函数去返回对应的参数
				//在rediscache返回的是domainuser,在dao层返回的是dao.user，所以在不同的expect处要返回不同的值
				c.EXPECT().Get(gomock.Any(), uid).
					Return(domain.User{}, cache.ErrKeyNotExist)
				d.EXPECT().FindById(gomock.Any(), uid).
					Return(dao.User{}, gorm.ErrRecordNotFound)
				//即从数据中找到了，然后写入cache中
				return c, d
			},
			ctx:      context.Background(),
			uid:      1,
			wantUser: domain.User{},
			wantErr:  gorm.ErrRecordNotFound,
		},
		{
			name: "回写缓存失败",
			mock: func(ctrl *gomock.Controller) (cache.UserCache, dao.UserDAO) {
				uid := int64(1)
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				//每一次返回的对象是不一样的，要根据写的函数去返回对应的参数
				//在rediscache返回的是domainuser,在dao层返回的是dao.user，所以在不同的expect处要返回不同的值
				c.EXPECT().Get(gomock.Any(), uid).
					Return(domain.User{}, cache.ErrKeyNotExist)
				d.EXPECT().FindById(gomock.Any(), uid).
					Return(dao.User{
						Id: uid,
						Email: sql.NullString{
							String: "123@qq.com",
							Valid:  true,
						},
						Password: "123456",
						Phone: sql.NullString{
							String: "17376913117",
							Valid:  true,
						},
						Ctime: nowMs,
						Utime: 102,
					}, nil)
				c.EXPECT().Set(gomock.Any(), domain.User{
					Id:       uid,
					Email:    "123@qq.com",
					Password: "123456",
					Phone:    "17376913117",
					Ctime:    now,
				}).Return(errors.New("redis错误"))
				//只是回写redis失败，不用在repository返回错误，记录日志即可
				return c, d
			},
			ctx: context.Background(),
			uid: 1,
			wantUser: domain.User{
				Id:       1,
				Email:    "123@qq.com",
				Password: "123456",
				Phone:    "17376913117",
				Ctime:    now,
			},
			//redis回写错误不用返回到repository层
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			UserCache, UserDao := tc.mock(ctrl)
			repo := NewUserRepository(UserDao, UserCache)
			user, err := repo.FindById(tc.ctx, tc.uid)
			assert.Equal(t, tc.wantUser, user)
			assert.Equal(t, tc.wantErr, err)

		})

	}
}
