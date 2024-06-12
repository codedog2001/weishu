package dao

import (
	"context"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"testing"
)

// 测试dao层需要利用sqldb来模拟真实的数据库，不然前提的数据处理是比较麻烦的
func TestGORMUserDAO_Insert(t *testing.T) {
	testCases := []struct {
		name    string
		mock    func(t *testing.T) *sql.DB
		ctx     context.Context
		user    User
		wantErr error
	}{
		{
			name: "insert success",
			mock: func(t *testing.T) *sql.DB {
				db, mock, err := sqlmock.New()
				assert.NoError(t, err)
				mockRes := sqlmock.NewResult(123, 1)
				mock.ExpectExec("INSERT INTO .*").WillReturnResult(mockRes)
				return db
			},
			ctx: context.Background(),
			user: User{
				Nickname: "TOM",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			//具体的测试代码
			sqlDB := tc.mock(t)
			db, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
				SkipDefaultTransaction:                   true,
			})
			assert.NoError(t, err)
			dao := NewUserDAO(db)
			err = dao.Insert(tc.ctx, tc.user)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
