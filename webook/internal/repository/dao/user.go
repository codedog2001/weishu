package dao

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"time"
)

var (
	ErrUserDuplicateEmail = errors.New("邮箱冲突")
	ErrRecordNotFound     = gorm.ErrRecordNotFound
)

type User struct {
	Id            int64          `gorm:"primaryKey,autoIncrement"`
	Email         sql.NullString `gorm:"unique"`
	Password      string
	Phone         sql.NullString `gorm:"unique"`
	Ctime         int64
	Utime         int64
	Nickname      string
	WechatUnionID sql.NullString
	WechatOpenID  sql.NullString `gorm:"unique"`
	//唯一索引允许有多个空值
	//但是不能有多个“” 空字符串
}
type UserDAO interface {
	Insert(ctx context.Context, u User) error
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByPhone(ctx context.Context, Phone string) (User, error)
	FindById(ctx context.Context, id int64) (User, error)
	FindByWechat(ctx context.Context, openID string) (User, error)
}

func (dao *GORMUserDAO) FindByWechat(ctx context.Context, openID string) (User, error) {
	var u User
	//取到的数据放在u里面
	err := dao.db.WithContext(ctx).Where("wechat_open_id=?", openID).First(&u).Error
	//err:=dao.db.WithContext(ctx).First(&u,"email=?",email).Error
	//两种写法都可以
	//如果err= gorm.ErrRecordNotFound,那么就会自动返回errusernotfound
	return u, err
}

type GORMUserDAO struct {
	db *gorm.DB
}

// NewUserDAO new函数都只是做一个初始化操作
func NewUserDAO(db *gorm.DB) UserDAO {
	return &GORMUserDAO{
		db: db,
	}
}
func (dao *GORMUserDAO) Insert(ctx context.Context, u User) error {
	now := time.Now().UnixMilli() //毫秒数在高并发的场景下更有优势
	u.Ctime = now
	u.Utime = now
	err := dao.db.WithContext(ctx).Create(&u).Error
	var mysqlErr *mysql.MySQLError
	//先判断错误是不是由mysql引起的，是的话再判断是不是由唯一索引错误引起的
	if errors.As(err, &mysqlErr) {
		const uniqueConflictsErrNo uint16 = 1062
		if mysqlErr.Number == uniqueConflictsErrNo {
			//只设置一个unique邮箱，所以发送唯一索引冲突的时候，就一定是邮箱出问题了
			return ErrUserDuplicateEmail
		}
	}
	return err //gorm会去数据库中创建u的实列

}

func (dao *GORMUserDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var u User
	//取到的数据放在u里面
	err := dao.db.WithContext(ctx).Where("email=?", email).First(&u).Error
	//err:=dao.db.WithContext(ctx).First(&u,"email=?",email).Error
	//两种写法都可以
	//如果err= gorm.ErrRecordNotFound,那么就会自动返回errusernotfound
	return u, err
}
func (dao *GORMUserDAO) FindByPhone(ctx context.Context, Phone string) (User, error) {
	var u User
	//取到的数据放在u里面
	err := dao.db.WithContext(ctx).Where("Phone=?", Phone).First(&u).Error
	//err:=dao.db.WithContext(ctx).First(&u,"email=?",email).Error
	//两种写法都可以
	//如果err= gorm.ErrRecordNotFound,那么就会自动返回errusernotfound
	return u, err
}
func (dao *GORMUserDAO) FindById(ctx context.Context, id int64) (User, error) {
	var u User
	//取到的数据放在u里面
	err := dao.db.WithContext(ctx).Where("email=?", id).First(&u).Error
	//err:=dao.db.WithContext(ctx).First(&u,"email=?",email).Error
	//两种写法都可以
	//如果err= gorm.ErrRecordNotFound,那么就会自动返回errusernotfound
	return u, err
}
