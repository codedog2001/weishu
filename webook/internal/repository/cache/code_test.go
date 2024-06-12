package cache

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"testing"
	"xiaoweishu/webook/internal/repository/cache/redismocks"
)

func TestRedisCodeCache_Set(t *testing.T) {
	keyFunc := func(biz, phone string) string {
		return fmt.Sprintf("phone_code:%s:%s", biz, phone)
	}
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) redis.Cmdable
		ctx     context.Context
		biz     string
		phone   string
		code    string
		wantErr error
	}{
		{
			name: "设置成功",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				res := redismocks.NewMockCmdable(ctrl)
				cmd := redis.NewCmd(context.Background())
				cmd.SetErr(nil)      //模拟执行Lua脚本时没有遇到错误
				cmd.SetVal(int64(0)) //模拟执行Lua脚本后返回整数0作为结果
				res.EXPECT().Eval(gomock.Any(), luaSetCode,
					[]string{keyFunc("test", "17376913117")},
					[]any{"123456"}).Return(cmd)
				return res
			},
			ctx:     context.Background(),
			biz:     "test",
			phone:   "17376913117",
			code:    "123456",
			wantErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			//具体的测试代码
			//先建立起控制器
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			//再建立模拟对象
			client := tc.mock(ctrl)
			//再建立被测对象，
			c := NewCodeCache(client)
			err := c.Set(tc.ctx, tc.biz, tc.phone, tc.code)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
