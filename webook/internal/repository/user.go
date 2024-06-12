package repository

import (
	"context"
	"database/sql"
	"time"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository/cache"
	"xiaoweishu/webook/internal/repository/dao"
)

var ErrUserDuplicateEmail = dao.ErrUserDuplicateEmail
var ErrUserNotFound = dao.ErrRecordNotFound

type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	FindById(ctx context.Context, id int64) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByPhone(ctx context.Context, phone string) (domain.User, error)
	FindByWechat(ctx context.Context, openID string) (domain.User, error)
}

func (r *CacheUserRepository) FindByWechat(ctx context.Context, openID string) (domain.User, error) {
	u, err := r.dao.FindByWechat(ctx, openID)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(u), nil
}

// CacheUserRepository 带有缓存的repository
type CacheUserRepository struct {
	dao   dao.UserDAO
	cache cache.UserCache
}

// NewUserRepository 要用的东西都不要内部初始化，让他从外面传入参数后调用new方法来进行初始化
func NewCacheUserRepository(dao dao.UserDAO, c cache.UserCache) UserRepository {
	return &CacheUserRepository{
		dao:   dao,
		cache: c,
	}
}

// Create repository已经是到达了数据库层面，所以这里不再会有注册的概念，而是要涉及到数据库的操作，所以这里写create
func (r *CacheUserRepository) Create(ctx context.Context, u domain.User) error {
	return r.dao.Insert(ctx, r.domainToEntity(u))
	//在这里操作缓存
}

// FindById 只要error为nil，就认为缓存里有数据
// 如果没有数据，就返回一个特定的error
func (r *CacheUserRepository) FindById(ctx context.Context, id int64) (domain.User, error) {
	//缓存里有数据
	//缓存没数据、
	//我也不知道有没有数据
	u, err := r.cache.Get(ctx, id)
	if err == nil {
		//必然是有数据
		return u, nil
	}
	//走到这说明redis未命中，继续往下到数据库中去找
	ue, err := r.dao.FindById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	u = r.entityToDomain(ue)
	//取到数据之后应该先放回去缓存中，方便后面的存取命中
	err = r.cache.Set(ctx, u)
	if err != nil {
		//这里不需要返回，设置缓存失败不是很大的事情
		//这里应该写日志，做监控，看看是不是redis崩掉了
	}
	//只要用到缓存就一定会出现一致性问题，所以这里可以用协程，这样不会阻碍主线程的进行
	return u, nil

}

func (r *CacheUserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := r.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(u), nil

}
func (r *CacheUserRepository) FindByPhone(ctx context.Context, phone string) (domain.User, error) {
	u, err := r.dao.FindByPhone(ctx, phone)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(u), nil

}
func (r *CacheUserRepository) entityToDomain(u dao.User) domain.User {
	return domain.User{
		Id:       u.Id,
		Email:    u.Email.String,
		Password: u.Password,
		Ctime:    time.UnixMilli(u.Ctime),
		Phone:    u.Phone.String,
		WechatInfo: domain.WechatInfo{
			UnionId: u.WechatUnionID.String,
			OpenId:  u.WechatOpenID.String,
		},
	}
}
func (r *CacheUserRepository) domainToEntity(u domain.User) dao.User {
	return dao.User{
		Id: u.Id,
		Email: sql.NullString{
			String: u.Email,
			Valid:  u.Email != "", //意思是当email不为空时，valid=true 即有效
		},
		Phone: sql.NullString{
			String: u.Phone,
			Valid:  u.Phone != "",
		},
		Password: u.Password,
		Ctime:    u.Ctime.UnixMilli(),
		WechatOpenID: sql.NullString{
			String: u.WechatInfo.UnionId,
			Valid:  u.WechatInfo.UnionId != "",
		},
		WechatUnionID: sql.NullString{
			String: u.WechatInfo.OpenId,
			Valid:  u.WechatInfo.OpenId != "",
		},
	}
}
